package state_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/effects/state"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateEffect_BasicLookup(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(store state.StateStore) {
		ctx, endOfStateHandler := state.WithEffectHandler(
			ctx,
			1, 1,
			false,
			store,
			map[string]any{
				"foo": 123,
			},
		)
		defer endOfStateHandler()

		v, err := state.EffectLoad[string, int](ctx, "foo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 123 {
			t.Fatalf("expected 123, got %v", v)
		}
	}

	for _, store := range []state.StateStore{
		state.NewInMemoryStore[string](),
		newTestSetStore[string](),
	} {
		testFn(store)
	}
}

func TestStateEffect_KeyNotFound(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(store state.StateStore) {
		ctx, endOfStateHandler := state.WithEffectHandler(
			ctx,
			1, 1,
			false,
			store,
			map[string]int{
				"foo": 123,
			},
		)
		defer endOfStateHandler()

		_, err := state.EffectLoad[string, int](ctx, "bar")
		if err == nil || !strings.Contains(err.Error(), "key not found") {
			t.Fatalf("expected key-not-found error, got: %v", err)
		}
	}

	for _, store := range []state.StateStore{
		state.NewInMemoryStore[string](),
		newTestSetStore[string](),
	} {
		testFn(store)
	}
}

func TestStateEffect_DelegatesToUpperScope(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(store state.StateStore) {
		upperCtx, upperClose := state.WithEffectHandler(
			ctx,
			1, 1,
			false,
			store,
			map[string]any{
				"upper": "delegated",
			},
		)
		defer upperClose()

		lowerCtx := context.WithValue(upperCtx, "dummy", 1)
		lowerCtx, lowerClose := state.WithEffectHandler[string, any](
			lowerCtx,
			1, 1,
			true,
			store,
			nil,
		)
		defer lowerClose()

		v, err := state.EffectLoad[string, any](lowerCtx, "upper")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "delegated" {
			t.Fatalf("expected delegated, got %v", v)
		}
	}

	for _, store := range []state.StateStore{state.NewInMemoryStore[string](), newTestSetStore[string]()} {
		testFn(store)
	}
}

func TestStateEffect_ConcurrentPartitionedAccess(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(store state.StateStore) {
		// prepare key-value map
		states := make(map[string]any)
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("key%d", i)
			states[key] = fmt.Sprintf("value%d", i)
		}

		// register the state handler with partitioning
		ctx, endOfStateHandler := state.WithEffectHandler(
			ctx,
			10, 10,
			false,
			store,
			states,
		)
		defer endOfStateHandler()

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

				v, err := state.EffectLoad[string, string](ctx, key)
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

	for _, store := range []state.StateStore{state.NewInMemoryStore[string](), newTestSetStore[string]()} {
		testFn(store)
	}

}

func TestStateEffect_ConcurrentReadWriteMixed(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(store state.StateStore) {
		ctx, endOfStateHandler := state.WithEffectHandler(
			ctx,
			8, 8,
			false,
			store,
			map[string]any{
				"x": "init",
			})
		defer endOfStateHandler()

		var wg sync.WaitGroup
		numWorkers := 100
		wg.Add(numWorkers)

		for i := 0; i < numWorkers; i++ {
			go func(i int) {
				defer wg.Done()
				key := fmt.Sprintf("key%d", i%10)

				// Load current value
				v, _ := state.EffectLoad[string, any](ctx, key)

				switch i % 4 {
				case 0:
					// Store unconditionally
					_, err := state.EffectInsertIfAbsent[string, any](ctx, key, i)
					assert.NoError(t, err)

				case 1:
					// Compare and delete
					deleted, err := state.EffectCompareAndDelete(ctx, key, v)
					assert.NoError(t, err)
					// deleted is bool (true if successful)
					_ = deleted

				case 2:
					// Compare and swap
					swapped, err := state.EffectCompareAndSwap[string, any](ctx,
						key,
						v,
						fmt.Sprintf("val-%d", i),
					)
					assert.NoError(t, err)
					// swapped is bool
					_ = swapped

				case 3:
					// Just load again to add read load
					_, _ = state.EffectLoad[string, any](ctx, key)
				}
			}(i)
		}

		wg.Wait()
	}

	for _, store := range []state.StateStore{state.NewInMemoryStore[string](), newTestSetStore[string]()} {
		testFn(store)
	}
}

func TestStateEffect_ContextTimeout(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(store state.StateStore) {
		ctx, endOfStateHandler := state.WithEffectHandler[string, any](
			ctx,
			1, 1,
			false,
			store,
			nil,
		)
		defer endOfStateHandler()

		// simulate handler blocking by using a long operation in a goroutine (deliberately omitted)
		// instead we simulate cancel before call

		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 1*time.Nanosecond)
		defer timeoutCancel()

		time.Sleep(1 * time.Millisecond) // allow timeout to occur

		res, err := state.EffectLoad[string, any](timeoutCtx, "any")
		log.Effect(ctx, log.LogInfo, "final result", map[string]interface{}{
			"res": res,
			"err": err,
		})
		assert.Nil(t, res)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	}

	for _, store := range []state.StateStore{state.NewInMemoryStore[string](), newTestSetStore[string]()} {
		testFn(store)
	}

}

func TestStateEffect_SetAndGet(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(store state.StateStore) {
		ctx, endOfStateHandler := state.WithEffectHandler[string, int](
			ctx,
			1, 1,
			false,
			store,
			nil,
		)
		defer endOfStateHandler()

		old, _ := state.EffectLoad[string, int](ctx, "foo")
		inserted, err := state.EffectInsertIfAbsent(ctx, "foo", 777)
		require.NoError(t, err)
		require.True(t, inserted)
		defer log.Effect(ctx, log.LogInfo, "swapped", map[string]any{
			"old": old,
			"new": 777,
		})

		_, err = state.EffectLoad[string, int](ctx, "foo")
		assert.NoError(t, err)
	}

	for _, store := range []state.StateStore{state.NewInMemoryStore[string](), newTestSetStore[string]()} {
		testFn(store)
	}

}

func TestStateEffect_SourcePayloadReturnsSink(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(store state.StateStore) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		ctx, endOfStateHandler := state.WithEffectHandler[string, string](
			ctx,
			8, 8,
			false,
			store,
			nil,
		)
		defer endOfStateHandler()

		// 1. Get src channel from SourceStatePayload
		src, err := state.EffectSource(ctx)
		require.NoError(t, err)

		// 2. Send a InsertPaylod
		key := "test-key"
		newVal := "new-value"
		inserted, err := state.EffectInsertIfAbsent(ctx, key, newVal)
		require.NoError(t, err)
		require.True(t, inserted)

		// 3. Check the sink channel for the StoreStatePayload
		select {
		case payload := <-src:
			storePayload, ok := payload.Payload.(state.InsertIfAbsent[string, string])
			require.True(t, ok)
			assert.Equal(t, storePayload.Key, key)
			assert.Equal(t, storePayload.New, newVal)
		case <-time.After(1 * time.Second):
			t.Fatal("expected payload not received on sink channel")
		}

		// 4. Send a CompareAndDeleteStatePayload
		oldVal := "new-value"
		_, err = state.EffectCompareAndDelete(ctx, key, oldVal)
		require.NoError(t, err)

		// 5. Check the sink channel for the CompareAndDeleteStatePayload
		select {
		case payload := <-src:
			storePayload, ok := payload.Payload.(state.CompareAndDelete[string, string])
			require.True(t, ok)
			assert.Equal(t, storePayload.Key, key)
			assert.Equal(t, storePayload.Old, oldVal)
		case <-time.After(1 * time.Second):
			t.Fatal("expected payload not received on sink channel")
		}
	}

	for _, store := range []state.StateStore{
		state.NewInMemoryStore[string](),
		// todo newTestSetStore[string](),
	} {
		testFn(store)
	}
}

func TestStore_Delegation(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(store state.StateStore) {
		// Tier 2 - has key x=1
		ctx2, endOfTier2 := state.WithEffectHandler(
			ctx,
			2, 2,
			false,
			state.NewInMemoryStore[string](),
			map[string]int{},
		)
		defer endOfTier2()

		// Tier 1 - has key x=1
		ctx1, endOfTier1 := state.WithEffectHandler(
			ctx2,
			2, 2,
			false,
			state.NewInMemoryStore[string](),
			map[string]int{},
		)
		defer endOfTier1()

		// Tier 0 - has key x=1
		ctx0, endOfTier0 := state.WithEffectHandler(
			ctx1,
			2, 2,
			true,
			store,
			map[string]int{},
		)
		defer endOfTier0()

		// Store should succeed in tier 0
		_, err := state.EffectInsertIfAbsent(ctx0, "x", 2)
		assert.NoError(t, err, "Store delegation failed")

		// Confirm that new value is set
		val, err := state.EffectLoad[string, int](ctx0, "x")
		assert.NoError(t, err, "Load failed")
		assert.Equal(t, val, 2, "Expected x=2")

		// Confirm that new value is set
		val, err = state.EffectLoad[string, int](ctx1, "x")
		assert.NoError(t, err, "Load failed")
		assert.Equal(t, val, 2, "Expected x=2")

		// Confirm that prev value is set
		_, err = state.EffectLoad[string, int](ctx2, "x")
		assert.ErrorIs(t, err, state.ErrNoSuchKey, "should be no such key")
	}

	for _, store := range []state.StateStore{state.NewInMemoryStore[string](), newTestSetStore[string]()} {
		testFn(store)
	}

}

func TestCompareAndSwap_Delegation(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(store state.StateStore) {
		// Tier 2 - has key x=1
		ctx2, endOfTier2 := state.WithEffectHandler(
			ctx,
			2, 2,
			false,
			state.NewInMemoryStore[string](),
			map[string]int{
				"x": 1,
			},
		)
		defer endOfTier2()

		// Tier 1 - has key x=1
		ctx1, endOfTier1 := state.WithEffectHandler(
			ctx2,
			2, 2,
			false,
			state.NewInMemoryStore[string](),
			map[string]int{
				"x": 1,
			},
		)
		defer endOfTier1()

		// Tier 0 - has key x=1
		ctx0, endOfTier0 := state.WithEffectHandler(
			ctx1,
			2, 2,
			true,
			store,
			map[string]int{
				"x": 1,
			},
		)
		defer endOfTier0()

		// CAS should succeed in tier 0
		ok, err := state.EffectCompareAndSwap(ctx0, "x", 1, 2)
		assert.NoError(t, err, "CAS delegation failed")
		assert.True(t, ok, "CAS delegation returned false, expected true")

		// Confirm that new value is set
		val, err := state.EffectLoad[string, int](ctx0, "x")
		assert.NoError(t, err, "Load failed")
		assert.Equal(t, val, 2, "Expected x=2")

		// Confirm that new value is set
		val, err = state.EffectLoad[string, int](ctx1, "x")
		assert.NoError(t, err, "Load failed")
		assert.Equal(t, val, 2, "Expected x=2")

		// Confirm that prev value is set
		val, err = state.EffectLoad[string, int](ctx2, "x")
		assert.NoError(t, err, "Load failed")
		assert.Equal(t, val, 1, "Expected x=1")
	}

	for _, store := range []state.StateStore{
		state.NewInMemoryStore[string](),
		// todo newTestSetStore[string](),
	} {
		testFn(store)
	}

}

func TestCompareAndDelete_Delegation(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(store state.StateStore) {
		// Tier 2 - has key y=99
		ctx2, endOfTier2 := state.WithEffectHandler(
			ctx,
			2, 2,
			false,
			state.NewInMemoryStore[string](),
			map[string]int{
				"y": 98,
			},
		)
		defer endOfTier2()

		// Tier 1 - has key y=99
		ctx1, endOfTier1 := state.WithEffectHandler(
			ctx2,
			2, 2,
			false,
			state.NewInMemoryStore[string](),
			map[string]int{
				"y": 99,
			},
		)
		defer endOfTier1()

		// Tier 0 - has key y=99
		ctx0, endOfTier0 := state.WithEffectHandler(
			ctx1,
			2, 2,
			true,
			store,
			map[string]int{
				"y": 99,
			},
		)
		defer endOfTier0()

		// CAD should succeed in tier 0
		ok, err := state.EffectCompareAndDelete(ctx0, "y", 99)
		assert.NoError(t, err, "CAD delegation failed")
		assert.True(t, ok, "CAD delegation returned false, expected true")

		// Confirm that prev value is set
		val, err := state.EffectLoad[string, int](ctx2, "y")
		assert.NoError(t, err, "Load failed")
		assert.Equal(t, val, 98, "Expected y=99")
	}

	for _, store := range []state.StateStore{state.NewInMemoryStore[string](), newTestSetStore[string]()} {
		testFn(store)
	}

}

func TestInsertIfAbsent_DelegationConflict_CAS_Success(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	// Upper tier with key "conflict" already set to "old"
	upperRepo := state.NewInMemoryStore[string]()
	upperCtx, endUpper := state.WithEffectHandler(ctx, 1, 1, false, upperRepo, map[string]string{
		"conflict": "old",
	})
	defer endUpper()

	// Lower tier (cache), delegation enabled
	cacheRepo := state.NewInMemoryStore[string]()
	cacheCtx, endCache := state.WithEffectHandler[string, string](upperCtx, 1, 1, true, cacheRepo, nil)
	defer endCache()

	// Insert with different value → should trigger CAS
	ok, err := state.EffectInsertIfAbsent(cacheCtx, "conflict", "new")
	assert.True(t, ok)
	assert.NoError(t, err)

	// Upper value should be updated to "new"
	v, err := state.EffectLoad[string, string](upperCtx, "conflict")
	assert.NoError(t, err)
	assert.Equal(t, "new", v)
}

func TestInsertIfAbsent_DelegationConflict_CAS_Success_2(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	// Upper tier starts with "x" = "v1"
	upperRepo := state.NewInMemoryStore[string]()
	upperCtx, endUpper := state.WithEffectHandler(ctx, 1, 1, false, upperRepo, map[string]string{
		"x": "v1",
	})
	defer endUpper()

	// Lower tier (cache), delegation enabled
	cacheRepo := state.NewInMemoryStore[string]()
	cacheCtx, endCache := state.WithEffectHandler[string, string](upperCtx, 1, 1, true, cacheRepo, nil)
	defer endCache()

	// Set up sync to force external CAS to run before retry
	ready := make(chan struct{})
	done := make(chan struct{})

	go func() {
		<-ready
		_, _ = state.EffectCompareAndSwap(upperCtx, "x", "v1", "external")
		close(done)
	}()

	// Trigger InsertIfAbsent, with delayed CAS
	close(ready)
	ok, err := state.EffectInsertIfAbsent(cacheCtx, "x", "v2")
	assert.True(t, ok)
	assert.NoError(t, err)

	// Wait for external CAS to finish
	<-done

	// Final value should be "external", not "v2"
	v, err := state.EffectLoad[string, string](upperCtx, "x")
	assert.NoError(t, err)
	assert.Equal(t, "v2", v)
}

func TestInsertIfAbsent_DelegationConflict_EqualValues_Skip(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	// Upper tier has the same value
	upperRepo := state.NewInMemoryStore[string]()
	upperCtx, endUpper := state.WithEffectHandler(ctx, 1, 1, false, upperRepo, map[string]string{
		"idempotent": "same",
	})
	defer endUpper()

	cacheRepo := state.NewInMemoryStore[string]()
	cacheCtx, endCache := state.WithEffectHandler[string, string](upperCtx, 1, 1, true, cacheRepo, nil)
	defer endCache()

	ok, err := state.EffectInsertIfAbsent(cacheCtx, "idempotent", "same")
	assert.True(t, ok)
	assert.NoError(t, err)

	v, err := state.EffectLoad[string, string](upperCtx, "idempotent")
	assert.NoError(t, err)
	assert.Equal(t, "same", v)
}

type testSetStore[K comparable] struct {
	*sync.Map
}

func newTestSetStore[K comparable]() state.StateStore {
	return state.NewSetStore(testSetStore[K]{Map: &sync.Map{}})
}

func (t testSetStore[K]) Get(key K) (value any, ok bool, err error) {
	value, ok = t.Load(key)
	return
}

func (t testSetStore[K]) Set(key K, value any) error {
	t.Store(key, value)
	return nil
}

func (t testSetStore[K]) Delete(key K) error {
	t.Map.Delete(key)
	return nil
}
