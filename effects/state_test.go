package effects_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestStateEffect_BasicLookup(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, closeFn := effects.WithStateEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		map[string]any{
			"foo": 123,
		},
	)
	defer closeFn()

	v, err := effects.StateEffect(ctx, effects.GetStatePayload{Key: "foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 123 {
		t.Fatalf("expected 123, got %v", v)
	}
}

func TestStateEffect_KeyNotFound(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()
	ctx, closeFn := effects.WithStateEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		map[string]any{
			"foo": 123,
		},
	)
	defer closeFn()

	_, err := effects.StateEffect(ctx, effects.GetStatePayload{Key: "bar"})
	if err == nil || !strings.Contains(err.Error(), "key not found") {
		t.Fatalf("expected key-not-found error, got: %v", err)
	}
}

func TestStateEffect_DelegatesToUpperScope(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	upperCtx, upperClose := effects.WithStateEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		map[string]any{
			"upper": "delegated",
		},
	)
	defer upperClose()

	lowerCtx := context.WithValue(upperCtx, "dummy", 1)
	lowerCtx, lowerClose := effects.WithStateEffectHandler(
		lowerCtx,
		effectmodel.NewEffectScopeConfig(1, 1),
		nil,
	)
	defer lowerClose()

	v, err := effects.StateEffect(lowerCtx, effects.GetStatePayload{Key: "upper"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "delegated" {
		t.Fatalf("expected delegated, got %v", v)
	}
}

func TestStateEffect_ConcurrentPartitionedAccess(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	// prepare key-value map
	states := make(map[string]any)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		states[key] = fmt.Sprintf("value%d", i)
	}

	// register the state handler with partitioning
	ctx, cancel := effects.WithStateEffectHandler(ctx, effectmodel.NewEffectScopeConfig(10, 10), states)
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
			keyIdx := i % len(states) // deterministic but shuffled
			key := fmt.Sprintf("key%d", keyIdx)

			v, err := effects.StateEffect(ctx, effects.GetStatePayload{Key: key})
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

var _ effects.Equatable = Int(0)

type Int int

func (i Int) Equals(other any) bool {
	if other == nil {
		return false
	}
	if otherInt, ok := other.(Int); ok {
		return i == otherInt
	}
	return false
}

func TestStateEffect_ConcurrentReadWriteMixed(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, cancel := effects.WithStateEffectHandler(ctx, effectmodel.NewEffectScopeConfig(8, 8), map[string]any{
		"x": "init",
	})
	defer cancel()

	var wg sync.WaitGroup
	numWorkers := 100
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i%10)

			switch i % 3 {
			case 0:
				effects.StateEffect(ctx, effects.SetStatePayload{Key: key, Value: Int(i)})
			case 1:
				effects.StateEffect(ctx, effects.DeleteStatePayload{Key: key})
			case 2:
				effects.StateEffect(ctx, effects.GetStatePayload{Key: key})
			}
		}(i)
	}

	wg.Wait()
}

func TestStateEffect_ContextTimeout(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, cancel := effects.WithStateEffectHandler(ctx, effectmodel.NewEffectScopeConfig(1, 1), nil)
	defer cancel()

	// simulate handler blocking by using a long operation in a goroutine (deliberately omitted)
	// instead we simulate cancel before call

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer timeoutCancel()

	time.Sleep(1 * time.Millisecond) // allow timeout to occur

	res, err := effects.StateEffect(timeoutCtx, effects.GetStatePayload{Key: "any"})
	if res != nil || err != nil {
		t.Fatal("expected timeout")
	}
}

func TestStateEffect_SetAndGet(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, cancel := effects.WithStateEffectHandler(ctx, effectmodel.NewEffectScopeConfig(1, 1), nil)
	defer cancel()

	_, err := effects.StateEffect(ctx, effects.SetStatePayload{Key: "foo", Value: Int(777)})
	if err != nil {
		t.Fatalf("failed to set: %v", err)
	}

	v, err := effects.StateEffect(ctx, effects.GetStatePayload{Key: "foo"})
	if err != nil {
		t.Fatalf("failed to get after set: %v", err)
	}
	if v != 777 {
		t.Fatalf("expected 777, got %v", v)
	}
}

func collectEvents(ctx context.Context, source <-chan effects.TimeBoundedStatePayload) []string {
	var result []string
	for {
		select {
		case ev, ok := <-source:
			if !ok {
				fmt.Println("channel closed")
				return result
			}
			switch payload := ev.StatePayload.(type) {
			case effects.SetStatePayload:
				effects.LogEffect(ctx, effects.LogDebug, "got event: Set", map[string]interface{}{"key": payload.Key})
				result = append(result, "Set "+payload.Key)
			case effects.DeleteStatePayload:
				effects.LogEffect(ctx, effects.LogDebug, "got event: Delete", map[string]interface{}{"key": payload.Key})
				result = append(result, "Delete "+payload.Key)
			default:
				effects.LogEffect(ctx, effects.LogDebug, "got unknown payload", map[string]interface{}{"payload": payload})
			}
		case <-time.After(500 * time.Millisecond):
			effects.LogEffect(ctx, effects.LogDebug, "timeout waiting for event", nil)
			return result
		}
	}
}

var defaultConfig = effectmodel.EffectScopeConfig{NumWorkers: 1, BufferSize: 1}

func TestStateSource_NoDelegation(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, endOfState := effects.WithStateEffectHandler(ctx, defaultConfig, nil)
	defer endOfState()

	source, _ := effects.StateEffect(ctx, effects.SourceStatePayload{})
	_, _ = effects.StateEffect(ctx, effects.SetStatePayload{Key: "a", Value: Int(1)})
	_, _ = effects.StateEffect(ctx, effects.DeleteStatePayload{Key: "a"})

	got := collectEvents(ctx, source.(chan effects.TimeBoundedStatePayload))
	want := []string{"Set a", "Delete a"}
	assert.ElementsMatch(t, want, got)
}

func TestStateSource_WithDelegation(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	// parent handler with a = 1
	parentCtx, parentEnd := effects.WithStateEffectHandler(ctx, defaultConfig, map[string]any{
		"a": 1,
	})
	defer parentEnd()

	// child handler with delegation
	childCtx, childEnd := effects.WithStateEffectHandler(parentCtx, defaultConfig, nil)
	source, _ := effects.StateEffect(ctx, effects.SourceStatePayload{})

	_, _ = effects.StateEffect(childCtx, effects.SetStatePayload{Key: "a", Value: Int(2)}) // override
	_, _ = effects.StateEffect(childCtx, effects.DeleteStatePayload{Key: "a"})             // delete locally

	ctx = childEnd() // close source

	got := collectEvents(ctx, source.(chan effects.TimeBoundedStatePayload))
	want := []string{"Set a", "Delete a"}
	assert.ElementsMatch(t, want, got)
}
