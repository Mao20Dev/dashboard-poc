package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"energy/internal/explain"
	"energy/internal/ingest"
	"energy/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAdmittedChatSurvivesDisconnectAndIsCached(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_chat_" + store.ID()
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); admin.Close() })
	db := &store.Store{Pool: pool, Timezone: "America/Bogota", ChatEnabled: true, ChatDailyLimit: 20}
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	in, err := ingest.Load("../../../data", db.Timezone)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Import(ctx, in); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		raw, _ := json.Marshal(explain.ChatAnswer{Answer: "Las lecturas permiten comparar la corriente.", Sources: []string{"meter:M-109"}})
		json.NewEncoder(w).Encode(map[string]any{"type": "message", "role": "assistant", "stop_reason": "end_turn", "content": []any{map[string]string{"type": "text", "text": string(raw)}}})
	}))
	defer server.Close()
	a := &API{DB: db, Providers: explain.Registry{"claude": &explain.Client{Provider: "claude", Key: "test", Model: "test-sonnet", BaseURL: server.URL}}}
	body := `{"message":"Explica la corriente de M-109","meter_id":"M-109","history":[]}`
	makeRequest := func(ctx context.Context) *http.Request {
		r := httptest.NewRequest("POST", "/api/v1/ai/chat", strings.NewReader(body)).WithContext(context.WithValue(ctx, actorKey, "test@local"))
		r.Header.Set("Content-Type", "application/json")
		return r
	}
	disconnected, cancel := context.WithCancel(ctx)
	defer cancel()
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { a.chat(w, makeRequest(disconnected)); close(done) }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		close(release)
		t.Fatal("provider never started")
	}
	cancel()
	close(release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("admitted request did not finish")
	}
	if w.Code != 200 {
		t.Fatal("disconnect discarded paid response", w.Code, w.Body.String())
	}
	second := httptest.NewRecorder()
	a.chat(second, makeRequest(ctx))
	var response ChatResponse
	if err = json.Unmarshal(second.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if second.Code != 200 || !response.Cached || calls.Load() != 1 {
		t.Fatal("cache caused another call", second.Code, calls.Load())
	}
}
