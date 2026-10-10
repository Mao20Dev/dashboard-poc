package store

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestChatReservationCacheFailureAndDailyLimit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	s.ChatEnabled = true
	s.ChatDailyLimit = 2
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.ReserveChat(ctx, "same", "actor")
			if e == nil {
				mu.Lock()
				accepted++
				mu.Unlock()
			} else if !errors.Is(e, ErrChatPending) {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if accepted != 1 {
		t.Fatal("duplicate paid admissions", accepted)
	}
	if e := s.FinishChat(ctx, "same", map[string]string{"answer": "saved"}, true); e != nil {
		t.Fatal(e)
	}
	s.ChatEnabled = false
	if raw, e := s.ReserveChat(ctx, "same", "actor"); e != nil || len(raw) == 0 {
		t.Fatal("cache blocked", e)
	}
	if _, e := s.ReserveChat(ctx, "disabled", "actor"); !errors.Is(e, ErrAIRequestsDisabled) {
		t.Fatal(e)
	}
	s.ChatEnabled = true
	if _, e := s.ReserveChat(ctx, "second", "actor"); e != nil {
		t.Fatal(e)
	}
	if e := s.FinishChat(ctx, "second", nil, false); e != nil {
		t.Fatal(e)
	}
	if _, e := s.ReserveChat(ctx, "second", "actor"); !errors.Is(e, ErrChatFailed) {
		t.Fatal("failed call retried", e)
	}
	if _, e := s.ReserveChat(ctx, "third", "actor"); !errors.Is(e, ErrChatLimit) {
		t.Fatal("daily cap bypassed", e)
	}
	if _, e := s.ReserveChat(ctx, "other-actor-key", "other"); e != nil {
		t.Fatal(e)
	}
}
