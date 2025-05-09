package task_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/effects/task"
)

func TestTaskEffect_Success(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, endOfTaskHandler := task.WithEffectHandler[string](ctx, 1)
	defer endOfTaskHandler()

	ch := task.Effect(ctx, func(ctx context.Context) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "ok", nil
	})

	select {
	case res := <-ch:
		if res.Err != nil {
			t.Fatalf("unexpected error: %v", res.Err)
		}
		if res.Value != "ok" {
			t.Fatalf("unexpected value: got %v", res.Value)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for task result")
	}
}

func TestTaskEffect_Cancelled(t *testing.T) {
	t.Skip("non-deterministic test. work in progress")
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	// Cancel after a little bit while.
	ctx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()

	ctx, endOfTaskHandler := task.WithEffectHandler[string](ctx, 1)
	defer endOfTaskHandler()

	block := make(chan struct{})

	ch := task.Effect(ctx, func(ctx context.Context) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(200 * time.Millisecond): // Besure after the timeout expired
			return "too late", nil
		}
	})

	// Unblock after 100ms (timeout expired at 20ms)
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(block)
	}()

	select {
	case res := <-ch:
		if !errors.Is(res.Err, context.DeadlineExceeded) {
			t.Fatalf("expected context cancellation error, got: %v", res.Err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for task result")
	}
}

func TestTaskEffect_Parallel(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, endOfTaskHandler := task.WithEffectHandler[int](ctx, 10)
	defer endOfTaskHandler()

	var results = make([]<-chan handlers.ResumableResult[int], 0)
	for i := 0; i < 5; i++ {
		n := i
		ch := task.Effect(ctx, func(ctx context.Context) (int, error) {
			time.Sleep(time.Duration(10+n*10) * time.Millisecond)
			return n * 2, nil
		})
		results = append(results, ch)
	}

	for i, ch := range results {
		select {
		case res := <-ch:
			if res.Err != nil {
				t.Fatalf("unexpected error in task %d: %v", i, res.Err)
			}
			expected := i * 2
			if res.Value != expected {
				t.Fatalf("unexpected result in task %d: got %d, want %d", i, res.Value, expected)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timeout waiting for task %d", i)
		}
	}
}
