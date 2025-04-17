package effects_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects"
)

func TestConcurrencyEffect_AllChildrenRunAndComplete(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx, endOfConcurrencyHandler := effects.WithConcurrencyEffectHandler(ctx, 10)
	defer endOfConcurrencyHandler()

	var mu sync.Mutex
	var ran []int
	var wg sync.WaitGroup
	wg.Add(3)

	f := func(i int) func(context.Context) {
		return func(ctx context.Context) {
			defer wg.Done()
			mu.Lock()
			ran = append(ran, i)
			mu.Unlock()
		}
	}

	effects.ConcurrencyEffect(ctx, []func(context.Context){
		f(1), f(2), f(3),
	})

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(ran) != 3 {
		t.Fatalf("expected 3 functions to run, got %v", ran)
	}
}

func TestConcurrencyEffect_ContextCancelPropagatesToChildren(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, cancel := context.WithCancel(ctx)

	ctx, endOfConcurrencyHandler := effects.WithConcurrencyEffectHandler(ctx, 10)
	defer endOfConcurrencyHandler()

	blocked := make(chan struct{})
	unblocked := make(chan struct{})

	effects.ConcurrencyEffect(ctx, []func(context.Context){
		func(ctx context.Context) {
			blocked <- struct{}{}
			<-ctx.Done()
			unblocked <- struct{}{}
		},
	})

	<-blocked
	cancel()

	select {
	case <-unblocked:
		// success
	case <-time.After(1 * time.Second):
		t.Fatal("expected child to unblock on context cancel")
	}
}

func TestConcurrencyEffect_HandlesPanicsGracefully(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, endOfConcurrencyHandler := effects.WithConcurrencyEffectHandler(ctx, 10)
	defer endOfConcurrencyHandler()

	done := make(chan struct{})
	effects.ConcurrencyEffect(ctx, []func(context.Context){
		func(ctx context.Context) {
			panic("child boom")
		},
		func(ctx context.Context) {
			done <- struct{}{}
		},
	})

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("expected non-panicking goroutine to finish")
	}
}

func TestConcurrencyEffect_WaitsUntilAllChildrenFinish(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, endOfConcurrencyHandler := effects.WithConcurrencyEffectHandler(ctx, 10)
	defer endOfConcurrencyHandler() // trigger waitChildren and block until goroutines finish

	done := make(chan struct{})

	wg := sync.WaitGroup{}
	numGoroutines := 5

	sleep100msAndDone := func(ctx context.Context) {
		defer wg.Done()
		time.Sleep(100 * time.Millisecond)
	}
	wg.Add(numGoroutines)
	effects.ConcurrencyEffect(ctx, []func(context.Context){
		sleep100msAndDone,
		sleep100msAndDone,
		sleep100msAndDone,
		sleep100msAndDone,
		sleep100msAndDone,
	})

	go func() {
		wg.Wait()
		close(done)
	}()
	// Wait for all goroutines to finish

	select {
	case <-done:
		// ok
	case <-time.After(1 * time.Second):
		t.Fatal("handler did not wait for children to complete")
	}

}
