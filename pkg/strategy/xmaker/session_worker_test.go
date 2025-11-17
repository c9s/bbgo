package xmaker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// helper waits until the condition is true or times out
func waitUntil(t *testing.T, cond func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", timeout)
}

func TestSessionWorker_StartOncePerWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := &sessionWorkerPool{workers: make(map[sessionWorkerKey]*sessionWorker)}

	var runs int32
	pool.Add(nil, "w1", func(ctx context.Context) {
		// simulate short work
		atomic.AddInt32(&runs, 1)
	})

	// Call Start multiple times; worker function should execute only once due to sync.Once
	pool.Start(ctx)
	pool.Start(ctx)

	waitUntil(t, func() bool { return atomic.LoadInt32(&runs) == 1 }, 500*time.Millisecond)
}

func TestSessionWorker_DistinctWorkersBothRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := &sessionWorkerPool{workers: make(map[sessionWorkerKey]*sessionWorker)}

	var a, b int32
	pool.Add(nil, "a", func(ctx context.Context) { atomic.AddInt32(&a, 1) })
	pool.Add(nil, "b", func(ctx context.Context) { atomic.AddInt32(&b, 1) })

	pool.Start(ctx)

	waitUntil(t, func() bool { return atomic.LoadInt32(&a) == 1 && atomic.LoadInt32(&b) == 1 }, 500*time.Millisecond)
}

func TestSessionWorker_OverwriteSameKeyOnlyNewestRunsOnStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := &sessionWorkerPool{workers: make(map[sessionWorkerKey]*sessionWorker)}

	var first, second int32

	// Add two workers with the same key (same session pointer and id)
	pool.Add(nil, "dup", func(ctx context.Context) { atomic.AddInt32(&first, 1) })
	pool.Add(nil, "dup", func(ctx context.Context) { atomic.AddInt32(&second, 1) })

	pool.Start(ctx)

	// Only the last added worker should be present and run
	waitUntil(t, func() bool { return atomic.LoadInt32(&first) == 1 }, 500*time.Millisecond)
	if got := atomic.LoadInt32(&second); got != 0 {
		t.Fatalf("expected first worker not to run, ran %d times", got)
	}
}
