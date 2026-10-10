package worker

import (
	"context"
	"energy/internal/domain"
	"energy/internal/engine"
	"energy/internal/explain"
	"energy/internal/store"
	"errors"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"time"
)

type Worker struct {
	Store     *store.Store
	Explainer *explain.Client
	Providers explain.Registry
}

func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		r, err := w.Store.Claim(ctx)
		if err == nil {
			w.process(ctx, r)
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) && ctx.Err() == nil {
			slog.Error("claim failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (w *Worker) process(parent context.Context, r domain.Run) {
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	defer func() {
		if v := recover(); v != nil {
			slog.Error("analysis panic", "run_id", r.ID)
			c, done := context.WithTimeout(context.Background(), 5*time.Second)
			defer done()
			_ = w.Store.Fail(c, r, "internal_error")
		}
	}()
	err := w.execute(ctx, r)
	if err != nil {
		slog.Error("analysis failed", "run_id", r.ID, "error", err)
		if parent.Err() != nil {
			return
		} // Leave the lease for another worker to recover after restart.
		c, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = w.Store.Fail(c, r, "analysis_failed")
	}
}
func (w *Worker) execute(ctx context.Context, r domain.Run) error {
	if err := w.Store.Stage(ctx, r, "DETECTION"); err != nil {
		return err
	}
	report, err := engine.Analyze(r.Input, engine.DefaultConfig())
	if err != nil {
		return err
	}
	if err = w.Store.Stage(ctx, r, "EXPLANATION"); err != nil {
		return err
	}
	explainer := w.Explainer
	if r.Provider != "" && r.Provider != "legacy" {
		configured, ok := w.Providers.Resolve(r.Provider)
		if !ok {
			configured = &explain.Client{Provider: r.Provider}
		}
		copyClient := *configured
		copyClient.Model = r.Model
		explainer = &copyClient
	}
	// One request for the entire plant. Never replay a paid attempt after recovery.
	if explainer != nil && explainer.Provider != "rules" && explainer.Key != "" && len(report.Findings) > 0 {
		if err = w.Store.ReserveProviderRequest(ctx, r); err != nil {
			return err
		}
	}
	explanations := explainer.ExplainBatch(ctx, report.Findings, r.Input)
	for i := range report.Findings {
		report.Findings[i].Explanation = explanations[i]
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = w.Store.Stage(ctx, r, "PERSISTENCE"); err != nil {
		return err
	}
	return w.Store.Complete(ctx, r, report)
}
