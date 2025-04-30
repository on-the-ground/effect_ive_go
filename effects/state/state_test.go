package state_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/effects/state"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateEffect_BasicLookup(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, closeFn := state.WithStateEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		false,
		map[string]any{
			"foo": 123,
		},
	)
	defer closeFn()

	v, err := state.StateEffect(ctx, state.LoadStatePayload{Key: "foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 123 {
		t.Fatalf("expected 123, got %v", v)
	}
}

func TestStateEffect_KeyNotFound(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()
	ctx, closeFn := state.WithStateEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		false,
		map[string]any{
			"foo": 123,
		},
	)
	defer closeFn()

	_, err := state.StateEffect(ctx, state.LoadStatePayload{Key: "bar"})
	if err == nil || !strings.Contains(err.Error(), "key not found") {
		t.Fatalf("expected key-not-found error, got: %v", err)
	}
}

func TestStateEffect_DelegatesToUpperScope(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	upperCtx, upperClose := state.WithStateEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		false,
		map[string]any{
			"upper": "delegated",
		},
	)
	defer upperClose()

	lowerCtx := context.WithValue(upperCtx, "dummy", 1)
	lowerCtx, lowerClose := state.WithStateEffectHandler(
		lowerCtx,
		effectmodel.NewEffectScopeConfig(1, 1),
		true,
		nil,
	)
	defer lowerClose()

	v, err := state.StateEffect(lowerCtx, state.LoadStatePayload{Key: "upper"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "delegated" {
		t.Fatalf("expected delegated, got %v", v)
	}
}

func TestStateEffect_ConcurrentPartitionedAccess(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	// prepare key-value map
	states := make(map[string]any)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		states[key] = fmt.Sprintf("value%d", i)
	}

	// register the state handler with partitioning
	ctx, cancel := state.WithStateEffectHandler(ctx, effectmodel.NewEffectScopeConfig(10, 10), false, states)
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

			v, err := state.StateEffect(ctx, state.LoadStatePayload{Key: key})
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

func TestStateEffect_ConcurrentReadWriteMixed(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, cancel := state.WithStateEffectHandler(ctx, effectmodel.NewEffectScopeConfig(8, 8), false, map[string]any{
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

			// Load current value
			v, _ := state.StateEffect(ctx, state.LoadStatePayload{Key: key})

			switch i % 4 {
			case 0:
				// Store unconditionally
				_, err := state.StateEffect(ctx, state.StoreStatePayload{
					Key: key,
					New: i,
				})
				assert.NoError(t, err)

			case 1:
				// Compare and delete
				deleted, err := state.StateEffect(ctx, state.CompareAndDeleteStatePayload{
					Key: key,
					Old: v,
				})
				assert.NoError(t, err)
				// deleted is bool (true if successful)
				_ = deleted

			case 2:
				// Compare and swap
				swapped, err := state.StateEffect(ctx, state.CompareAndSwapStatePayload{
					Key: key,
					Old: v,
					New: fmt.Sprintf("val-%d", i),
				})
				assert.NoError(t, err)
				// swapped is bool
				_ = swapped

			case 3:
				// Just load again to add read load
				_, _ = state.StateEffect(ctx, state.LoadStatePayload{Key: key})
			}
		}(i)
	}

	wg.Wait()
}

func TestStateEffect_ContextTimeout(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, cancel := state.WithStateEffectHandler(ctx, effectmodel.NewEffectScopeConfig(1, 1), false, nil)
	defer cancel()

	// simulate handler blocking by using a long operation in a goroutine (deliberately omitted)
	// instead we simulate cancel before call

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer timeoutCancel()

	time.Sleep(1 * time.Millisecond) // allow timeout to occur

	res, err := state.StateEffect(timeoutCtx, state.LoadStatePayload{Key: "any"})
	log.LogEffect(ctx, log.LogInfo, "final result", map[string]interface{}{
		"res": res,
		"err": err,
	})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestStateEffect_SetAndGet(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, cancel := state.WithStateEffectHandler(ctx, effectmodel.NewEffectScopeConfig(1, 1), false, nil)
	defer cancel()

	old, _ := state.StateEffect(ctx, state.LoadStatePayload{Key: "foo"})
	state.StateEffect(ctx, state.StoreStatePayload{Key: "foo", New: (777)})
	defer log.LogEffect(ctx, log.LogInfo, "swapped", map[string]any{
		"old": old,
		"new": 777,
	})

	v, err := state.StateEffect(ctx, state.LoadStatePayload{Key: "foo"})
	assert.NoError(t, err)
	assert.Equal(t, v, 777)
}

func TestStateEffect_SourcePayloadReturnsSink(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx, end := state.WithStateEffectHandler(ctx, effectmodel.NewEffectScopeConfig(8, 8), false, nil)
	defer end()

	// 1. Get sink channel from SourceStatePayload
	chVal, err := state.StateEffect(ctx, state.SourceStatePayload{})
	require.NoError(t, err)

	sink, ok := chVal.(chan state.TimeBoundedStatePayload)
	require.True(t, ok, "expected sink channel from SourceStatePayload")

	// 2. Send a StoreStatePayload
	key := "test-key"
	newVal := "new-value"
	_, err = state.StateEffect(ctx, state.StoreStatePayload{
		Key: key,
		New: newVal,
	})
	require.NoError(t, err)

	// 3. Check the sink channel for the StoreStatePayload
	select {
	case payload := <-sink:
		storePayload, ok := payload.StatePayload.(state.StoreStatePayload)
		require.True(t, ok)
		assert.Equal(t, storePayload.Key, key)
		assert.Equal(t, storePayload.New, newVal)
	case <-time.After(1 * time.Second):
		t.Fatal("expected payload not received on sink channel")
	}

	// 4. Send a CompareAndDeleteStatePayload
	oldVal := "new-value"
	_, err = state.StateEffect(ctx, state.CompareAndDeleteStatePayload{
		Key: key,
		Old: oldVal,
	})
	require.NoError(t, err)

	// 5. Check the sink channel for the CompareAndDeleteStatePayload
	select {
	case payload := <-sink:
		storePayload, ok := payload.StatePayload.(state.CompareAndDeleteStatePayload)
		require.True(t, ok)
		assert.Equal(t, storePayload.Key, key)
		assert.Equal(t, storePayload.Old, oldVal)
	case <-time.After(1 * time.Second):
		t.Fatal("expected payload not received on sink channel")
	}
}
