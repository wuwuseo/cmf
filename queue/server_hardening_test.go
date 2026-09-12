package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
)

func TestWorkerRejectsOversizedPayloadWithoutRetry(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxPayloadBytes = 8
	server := NewServer(cfg)
	called := false
	server.Handle("test", func(context.Context, *Task) error { called = true; return nil })
	err := server.mux.ProcessTask(t.Context(), asynq.NewTask("test", []byte("123456789")))
	if called || !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("oversized payload reached handler or retries: called=%v, err=%v", called, err)
	}
	if err := server.mux.ProcessTask(t.Context(), asynq.NewTask("test", []byte("12345678"))); err != nil || !called {
		t.Fatalf("boundary payload rejected: %v", err)
	}
}
