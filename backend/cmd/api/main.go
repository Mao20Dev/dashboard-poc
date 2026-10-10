package main

import (
	"bufio"
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"

	"energy/internal/api"
	"energy/internal/explain"
	"energy/internal/ingest"
	"energy/internal/store"
	"energy/internal/worker"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
func dotenv(path string) {
	f, e := os.Open(path)
	if e != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok && os.Getenv(strings.TrimSpace(k)) == "" {
			_ = os.Setenv(strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), "\"'"))
		}
	}
}
func main() {
	dotenv("../.env")
	dotenv(".env")
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	db, err := store.Open(startup, env("DATABASE_URL", "postgres://energy:energy-local-only@localhost:54329/energy?sslmode=disable"), env("PLANT_TZ", "America/Bogota"))
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Pool.Close()
	db.ChatEnabled = env("CHAT_ENABLED", "false") == "true"
	db.ChatDailyLimit, _ = strconv.Atoi(env("CHAT_DAILY_LIMIT", "20"))
	if db.ChatDailyLimit < 1 || db.ChatDailyLimit > 100 {
		db.ChatDailyLimit = 20
	}
	db.AllowNewAIRequests = env("AI_ALLOW_NEW_REQUESTS", "false") == "true"
	if err = db.Migrate(startup); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	input, err := ingest.Load(env("DATA_DIR", "../data"), db.Timezone)
	if err != nil {
		slog.Error("dataset validation failed", "error", err)
		os.Exit(1)
	}
	if err = db.Import(startup, input); err != nil {
		slog.Error("import failed", "error", err)
		os.Exit(1)
	}
	if err = db.EnsureUser(startup, env("DEMO_EMAIL", "uribe@energyiq.test"), env("DEMO_PASSWORD", "EnergyDemo2026!")); err != nil {
		slog.Error("demo user setup failed", "error", err)
		os.Exit(1)
	}
	providers := explain.Registry{
		"claude": &explain.Client{Provider: "claude", Key: os.Getenv("ANTHROPIC_API_KEY"), Model: env("ANTHROPIC_MODEL", explain.DefaultModel), WorkspaceID: os.Getenv("ANTHROPIC_WORKSPACE_ID")},
		"openai": &explain.Client{Provider: "openai", Key: os.Getenv("OPENAI_API_KEY"), Model: env("OPENAI_MODEL", explain.DefaultOpenAIModel)},
	}
	w := worker.Worker{Store: db, Providers: providers}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); w.Start(ctx) }()
	server := &http.Server{Addr: env("ADDR", "127.0.0.1:8080"), Handler: api.New(db, env("COOKIE_SECURE", "false") == "true", providers), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		slog.Info("api ready", "addr", server.Addr, "readings", len(input.Readings), "timezone", db.Timezone)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http failed", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	_ = server.Shutdown(shutdown)
	wg.Wait()
}
