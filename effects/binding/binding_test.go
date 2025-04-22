package binding_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/on-the-ground/effect_ive_go/effects/binding"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	"github.com/on-the-ground/effect_ive_go/effects/log"
)

func TestBindingEffect_BasicLookup(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, closeFn := binding.WithBindingEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		map[string]any{
			"foo": 123,
		},
	)
	defer closeFn()

	v, err := binding.BindingEffect(ctx, binding.BindingPayload{Key: "foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 123 {
		t.Fatalf("expected 123, got %v", v)
	}
}

func TestBindingEffect_KeyNotFound(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()
	ctx, closeFn := binding.WithBindingEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		map[string]any{
			"foo": 123,
		},
	)
	defer closeFn()

	_, err := binding.BindingEffect(ctx, binding.BindingPayload{Key: "bar"})
	if err == nil || !strings.Contains(err.Error(), "key not found") {
		t.Fatalf("expected key-not-found error, got: %v", err)
	}
}

func TestBindingEffect_DelegatesToUpperScope(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	upperCtx, upperClose := binding.WithBindingEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		map[string]any{
			"upper": "delegated",
		},
	)
	defer upperClose()

	lowerCtx := context.WithValue(upperCtx, "dummy", 1)
	lowerCtx, lowerClose := binding.WithBindingEffectHandler(
		lowerCtx,
		effectmodel.NewEffectScopeConfig(1, 1),
		nil,
	)
	defer lowerClose()

	v, err := binding.BindingEffect(lowerCtx, binding.BindingPayload{Key: "upper"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "delegated" {
		t.Fatalf("expected delegated, got %v", v)
	}
}

func TestBindingEffect_ConcurrentPartitionedAccess(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	// prepare key-value map
	bindings := make(map[string]any)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		bindings[key] = fmt.Sprintf("value%d", i)
	}

	// register the binding handler with partitioning
	ctx, cancel := binding.WithBindingEffectHandler(ctx, effectmodel.NewEffectScopeConfig(10, 10), bindings)
	defer cancel()

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make(map[string]int) // key => hit count
		errs    int
	)

	numRequests := 1000
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func(i int) {
			defer wg.Done()

			// pick a key randomly
			keyIdx := i % len(bindings) // deterministic but shuffled
			key := fmt.Sprintf("key%d", keyIdx)

			v, err := binding.BindingEffect(ctx, binding.BindingPayload{Key: key})
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				t.Errorf("unexpected error for key %s: %v", key, err)
				errs++
				return
			}
			expected := fmt.Sprintf("value%d", keyIdx)
			if v != expected {
				t.Errorf("unexpected value for key %s: got %v, want %v", key, v, expected)
			}
			results[key]++
		}(i)
	}

	wg.Wait()

	// verify that all keys were hit at least once
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		if count := results[key]; count == 0 {
			t.Errorf("key %s was never used", key)
		} else {
			t.Logf("key %s was used %d times", key, count)
		}
	}

	if errs > 0 {
		t.Errorf("Total error count: %d", errs)
	}
}
