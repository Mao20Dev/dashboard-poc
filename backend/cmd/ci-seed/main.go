// ci-seed creates synthetic provider caches for browser tests only. It never calls an LLM.
package main

import (
	"context"
	"energy/internal/domain"
	"energy/internal/engine"
	"energy/internal/explain"
	"energy/internal/ingest"
	"energy/internal/store"
	"fmt"
	"log"
	"os"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	if os.Getenv("CI") != "true" || os.Getenv("CI_SEED_DATABASE_URL") == "" {
		log.Fatal("CI=true and CI_SEED_DATABASE_URL required; never run against application data")
	}
	ctx := context.Background()
	s, err := store.Open(ctx, os.Getenv("CI_SEED_DATABASE_URL"), "America/Bogota")
	must(err)
	defer s.Pool.Close()
	must(s.Migrate(ctx))
	var count int
	must(s.Pool.QueryRow(ctx, "SELECT count(*) FROM analysis_runs").Scan(&count))
	if count != 0 {
		log.Fatal("refusing to overwrite existing analyses")
	}
	input, err := ingest.Load("../data", s.Timezone)
	must(err)
	must(s.Import(ctx, input))
	s.AllowNewAIRequests = true
	for _, provider := range []string{"rules", "openai", "claude"} {
		model := ""
		if provider == "openai" {
			model = explain.DefaultOpenAIModel
		}
		if provider == "claude" {
			model = explain.DefaultModel
		}
		_, err = s.EnqueueProvider(ctx, provider, model)
		must(err)
		run, err := s.Claim(ctx)
		must(err)
		report, err := engine.Analyze(run.Input, engine.DefaultConfig())
		must(err)
		for i := range report.Findings {
			f := &report.Findings[i]
			f.Explanation = (&explain.Client{Provider: "rules"}).Explain(ctx, *f)
			if provider != "rules" {
				f.Explanation.Source = provider
				f.Explanation.Model = model
				f.Explanation.Summary = fmt.Sprintf("Fixture de prueba %s: %s", provider, f.Explanation.Summary)
				f.Explanation.Headline = "Fixture: revisar las señales del medidor"
				f.Explanation.Interpretation = []domain.Guidance{{Code: "fixture", Text: "Interpretación sintética para verificar el cambio de proveedor sin llamadas externas.", EvidenceIDs: f.Explanation.SummaryEvidenceIDs}}
			}
		}
		must(s.Complete(ctx, run, report))
	}
	fmt.Println("CI fixtures ready: real measurements, synthetic provider narratives, zero LLM requests")
}
