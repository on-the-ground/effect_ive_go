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
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, endOfTaskHandler := task.WithTaskEffectHandler[string](ctx, 1)
	defer endOfTaskHandler()

	ch := task.TaskEffect(ctx, func(ctx context.Context) (string, error) {
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
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	// 타임아웃을 주고, 바로 취소하지 않음
	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	ctx, endOfTaskHandler := task.WithTaskEffectHandler[string](ctx, 1)
	defer endOfTaskHandler()

	block := make(chan struct{})

	ch := task.TaskEffect(ctx, func(ctx context.Context) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(200 * time.Millisecond): // 확실히 context보다 늦음
			return "too late", nil
		}
	})

	// 100ms 뒤에 unblock 하는 타이머 설정 (context는 50ms에 취소됨)
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(block)
	}()

	select {
	case res, ok := <-ch:
		if !ok || res.Err == nil || !errors.Is(res.Err, context.DeadlineExceeded) {
			t.Fatalf("expected context cancellation error, got: %v", res.Err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for task result")
	}
}

func TestTaskEffect_Parallel(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, endOfTaskHandler := task.WithTaskEffectHandler[int](ctx, 10)
	defer endOfTaskHandler()

	var results = make([]<-chan handlers.ResumableResult[int], 0)
	for i := 0; i < 5; i++ {
		n := i
		ch := task.TaskEffect(ctx, func(ctx context.Context) (int, error) {
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
