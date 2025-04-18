package effects_test

import (
	"context"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects"
	"github.com/stretchr/testify/assert"
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

	effects.ConcurrencyEffect(ctx, f(1), f(2), f(3))

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

	effects.ConcurrencyEffect(ctx,
		func(ctx context.Context) {
			blocked <- struct{}{}
			<-ctx.Done()
			unblocked <- struct{}{}
		},
	)

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
	effects.ConcurrencyEffect(ctx,
		func(ctx context.Context) {
			panic("child boom")
		},
		func(ctx context.Context) {
			done <- struct{}{}
		},
	)

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
	effects.ConcurrencyEffect(ctx,
		sleep100msAndDone,
		sleep100msAndDone,
		sleep100msAndDone,
		sleep100msAndDone,
		sleep100msAndDone,
	)

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

func TestConcurrencyEffect_SpawnsAndCleansUpAll(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, endOfConcurrencyHandler := effects.WithConcurrencyEffectHandler(ctx, 10)

	done := make(chan string, 4) // deterministic 수집
	numRoutines := 0

	record := func(name string) func(context.Context) {
		return func(ctx context.Context) {
			time.Sleep(10 * time.Millisecond) // simulate work
			numRoutines++
			done <- name
		}
	}

	// First round
	effects.ConcurrencyEffect(ctx,
		record("A1"),
		record("A2"),
	)

	// Second round (before previous is complete)
	effects.ConcurrencyEffect(ctx,
		record("B1"),
		record("B2"),
	)

	// End scope - should block until all goroutines complete
	ctx = endOfConcurrencyHandler()

	assert.Equal(t, numRoutines, 4, "Expected 4 goroutines to run")

	// Collect all 4 names
	var results []string
	for i := 0; i < 4; i++ {
		select {
		case name := <-done:
			effects.LogEffect(ctx, effects.LogInfo, "collected name", map[string]interface{}{
				"name": name,
			})
			results = append(results, name)
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("Timed out waiting for goroutine %d", i+1)
		}
	}

	// Check we got exactly 4 expected names
	sort.Strings(results)
	want := []string{"A1", "A2", "B1", "B2"}
	sort.Strings(want)

	if !slices.Equal(results, want) {
		t.Errorf("Expected %v, got %v", want, results)
	}
}
