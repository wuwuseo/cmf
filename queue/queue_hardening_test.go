package queue_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/wuwuseo/cmf/queue"
)

func TestAdmissionLimitsPayloadAndConcurrentQueueSize(t *testing.T) {
	flushTestDB(t)
	cfg := newTestConfig()
	cfg.MaxQueueSize, cfg.MaxPayloadBytes = 8, 8
	client := queue.NewClient(cfg)
	defer client.Close()
	if _, err := client.Enqueue(t.Context(), "test", []byte("中文中文")); !errors.Is(err, queue.ErrPayloadTooLarge) {
		t.Fatalf("byte limit not enforced: %v", err)
	}
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for n := 0; n < 32; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.Enqueue(t.Context(), "test", []byte("12345678"), queue.WithDelay(time.Hour))
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, queue.ErrQueueFull) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 8 {
		t.Fatalf("accepted %d, want 8", accepted.Load())
	}
	inspector := queue.NewInspector(cfg)
	defer inspector.Close()
	stats, err := inspector.QueueStats("default")
	if err != nil || stats.Scheduled != 8 {
		t.Fatalf("scheduled tasks bypassed admission: %+v, %v", stats, err)
	}
}

func TestDefaultQueueIsDeterministic(t *testing.T) {
	flushTestDB(t)
	for _, target := range []string{"default", "alpha"} {
		cfg := newTestConfig()
		cfg.Queues = map[string]int{target: 1, "zulu": 1}
		inspector := queue.NewInspector(cfg)
		for n := 0; n < 32; n++ {
			client := queue.NewClient(cfg)
			id, err := client.Enqueue(t.Context(), "test", nil)
			client.Close()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := inspector.GetTask(target, id); err != nil {
				t.Fatalf("default enqueue did not choose %q: %v", target, err)
			}
		}
		inspector.Close()
	}
}

func TestPruneArchivedMultipleBatchesPreservesNewAndRetriedTasks(t *testing.T) {
	flushTestDB(t)
	cfg := newTestConfig()
	rc := goredis.NewClient(&goredis.Options{Addr: cfg.Addr, Password: cfg.Password, DB: cfg.DB})
	defer rc.Close()
	ctx := t.Context()
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	const prefix = "asynq:{default}:"
	// Seed the pinned asynq v0.26 storage layout. No payload decoding should be needed by pruning.
	_, err := rc.Pipelined(ctx, func(p goredis.Pipeliner) error {
		for n := 0; n < 250; n++ {
			id := fmt.Sprintf("old-%03d", n)
			p.HSet(ctx, prefix+"t:"+id, "state", "archived", "msg", strings.Repeat("x", 1024))
			p.ZAdd(ctx, prefix+"archived", goredis.Z{Score: float64(cutoff.Add(-time.Hour).Unix()), Member: id})
		}
		p.HSet(ctx, prefix+"t:new", "state", "archived")
		p.ZAdd(ctx, prefix+"archived", goredis.Z{Score: float64(time.Now().Unix()), Member: "new"})
		p.HSet(ctx, prefix+"t:retried", "state", "pending")
		p.ZAdd(ctx, prefix+"archived", goredis.Z{Score: float64(cutoff.Unix()), Member: "retried"})
		p.Set(ctx, prefix+"unique", "old-000", 0)
		p.HSet(ctx, prefix+"t:old-000", "unique_key", prefix+"unique")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	inspector := queue.NewInspector(cfg)
	defer inspector.Close()
	removed, err := inspector.PruneArchivedContext(ctx, "default", cutoff)
	if err != nil || removed != 251 {
		t.Fatalf("removed=%d, err=%v", removed, err)
	}
	for n := 0; n < 250; n++ {
		if rc.Exists(ctx, fmt.Sprintf("%st:old-%03d", prefix, n)).Val() != 0 {
			t.Fatalf("old task %d skipped", n)
		}
	}
	if rc.Exists(ctx, prefix+"t:new", prefix+"t:retried").Val() != 2 {
		t.Fatal("new or retried task was deleted")
	}
	if rc.Exists(ctx, prefix+"unique").Val() != 0 {
		t.Fatal("uniqueness lock leaked")
	}
	if rc.ZCard(ctx, prefix+"archived").Val() != 1 {
		t.Fatal("archive index inconsistent")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := inspector.PruneArchivedContext(cancelCtx, "default", cutoff); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
}
