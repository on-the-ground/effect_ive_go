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

var _ state.SetRepo = testSetRepo{Map: &sync.Map{}}

type testSetRepo struct {
	*sync.Map
}

func newTestSetRepo() state.StateRepo {
	return state.NewSetRepo(testSetRepo{Map: &sync.Map{}})
}

func (t testSetRepo) Get(key any) (value any, ok bool) {
	return t.Load(key)
}

func (t testSetRepo) Set(key, value any) {
	t.Store(key, value)
}

var _ state.CasRepo = testCasRepo{Map: &sync.Map{}}

type testCasRepo struct {
	*sync.Map
}

func newTestCasRepo() state.StateRepo {
	return state.NewCasRepo(testCasRepo{Map: &sync.Map{}})
}

func (t testCasRepo) InsertIfAbsent(key, value any) (inserted bool) {
	_, loaded := t.LoadOrStore(key, value)
	return !loaded
}

func TestStateEffect_BasicLookup(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(repo state.StateRepo) {
		ctx, endOfStateHandler := state.WithEffectHandler(
			ctx,
			effectmodel.NewEffectScopeConfig(1, 1),
			false,
			repo,
			map[string]any{
				"foo": 123,
			},
		)
		defer endOfStateHandler()

		v, err := state.Effect(ctx, state.LoadPayloadOf("foo"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 123 {
			t.Fatalf("expected 123, got %v", v)
		}
	}

	for _, repo := range []state.StateRepo{
		newTestCasRepo(),
		newTestSetRepo(),
	} {
		testFn(repo)
	}
}

func TestStateEffect_KeyNotFound(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(repo state.StateRepo) {
		ctx, endOfStateHandler := state.WithEffectHandler(
			ctx,
			effectmodel.NewEffectScopeConfig(1, 1),
			false,
			repo,
			map[string]any{
				"foo": 123,
			},
		)
		defer endOfStateHandler()

		_, err := state.Effect(ctx, state.LoadPayloadOf("bar"))
		if err == nil || !strings.Contains(err.Error(), "key not found") {
			t.Fatalf("expected key-not-found error, got: %v", err)
		}
	}

	for _, repo := range []state.StateRepo{
		newTestCasRepo(),
		newTestSetRepo(),
	} {
		testFn(repo)
	}
}

func TestStateEffect_DelegatesToUpperScope(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(repo state.StateRepo) {
		upperCtx, upperClose := state.WithEffectHandler(
			ctx,
			effectmodel.NewEffectScopeConfig(1, 1),
			false,
			repo,
			map[string]any{
				"upper": "delegated",
			},
		)
		defer upperClose()

		lowerCtx := context.WithValue(upperCtx, "dummy", 1)
		lowerCtx, lowerClose := state.WithEffectHandler[string, any](
			lowerCtx,
			effectmodel.NewEffectScopeConfig(1, 1),
			true,
			repo,
			nil,
		)
		defer lowerClose()

		v, err := state.Effect(lowerCtx, state.LoadPayloadOf("upper"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "delegated" {
			t.Fatalf("expected delegated, got %v", v)
		}
	}

	for _, repo := range []state.StateRepo{newTestCasRepo()} {
		testFn(repo)
	}
}

func TestStateEffect_ConcurrentPartitionedAccess(t *testing.T) {
	ctx := context.Background()

	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(repo state.StateRepo) {
		// prepare key-value map
		states := make(map[string]any)
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("key%d", i)
			states[key] = fmt.Sprintf("value%d", i)
		}

		// register the state handler with partitioning
		ctx, endOfStateHandler := state.WithEffectHandler(
			ctx,
			effectmodel.NewEffectScopeConfig(10, 10),
			false,
			repo,
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

				v, err := state.Effect(ctx, state.LoadPayloadOf(key))
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

	for _, repo := range []state.StateRepo{newTestCasRepo()} {
		testFn(repo)
	}

}

func TestStateEffect_ConcurrentReadWriteMixed(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(repo state.StateRepo) {
		ctx, endOfStateHandler := state.WithEffectHandler(
			ctx,
			effectmodel.NewEffectScopeConfig(8, 8),
			false,
			repo,
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
				v, _ := state.Effect(ctx, state.LoadPayloadOf(key))

				switch i % 4 {
				case 0:
					// Store unconditionally
					_, err := state.Effect(ctx, state.InsertPayloadOf[string, any](key, i))
					assert.NoError(t, err)

				case 1:
					// Compare and delete
					deleted, err := state.Effect(ctx, state.CADPayloadOf(key, v))
					assert.NoError(t, err)
					// deleted is bool (true if successful)
					_ = deleted

				case 2:
					// Compare and swap
					swapped, err := state.Effect(ctx, state.CASPayloadOf[string, any](
						key,
						v,
						fmt.Sprintf("val-%d", i),
					))
					assert.NoError(t, err)
					// swapped is bool
					_ = swapped

				case 3:
					// Just load again to add read load
					_, _ = state.Effect(ctx, state.LoadPayloadOf(key))
				}
			}(i)
		}

		wg.Wait()
	}

	for _, repo := range []state.StateRepo{newTestCasRepo()} {
		testFn(repo)
	}
}

func TestStateEffect_ContextTimeout(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(repo state.StateRepo) {
		ctx, endOfStateHandler := state.WithEffectHandler[string, any](
			ctx,
			effectmodel.NewEffectScopeConfig(1, 1),
			false,
			repo,
			nil,
		)
		defer endOfStateHandler()

		// simulate handler blocking by using a long operation in a goroutine (deliberately omitted)
		// instead we simulate cancel before call

		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 1*time.Nanosecond)
		defer timeoutCancel()

		time.Sleep(1 * time.Millisecond) // allow timeout to occur

		res, err := state.Effect(timeoutCtx, state.LoadPayloadOf("any"))
		log.Effect(ctx, log.LogInfo, "final result", map[string]interface{}{
			"res": res,
			"err": err,
		})
		assert.Nil(t, res)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	}

	for _, repo := range []state.StateRepo{newTestCasRepo()} {
		testFn(repo)
	}

}

func TestStateEffect_SetAndGet(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(repo state.StateRepo) {
		ctx, endOfStateHandler := state.WithEffectHandler[string, int](
			ctx,
			effectmodel.NewEffectScopeConfig(1, 1),
			false,
			repo,
			nil,
		)
		defer endOfStateHandler()

		old, _ := state.Effect(ctx, state.LoadPayloadOf("foo"))
		state.Effect(ctx, state.InsertPayloadOf("foo", 777))
		defer log.Effect(ctx, log.LogInfo, "swapped", map[string]any{
			"old": old,
			"new": 777,
		})

		_, err := state.Effect(ctx, state.LoadPayloadOf("foo"))
		assert.NoError(t, err)
	}

	for _, repo := range []state.StateRepo{newTestCasRepo()} {
		testFn(repo)
	}

}

func TestStateEffect_SourcePayloadReturnsSink(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(repo state.StateRepo) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		ctx, endOfStateHandler := state.WithEffectHandler[string, string](
			ctx,
			effectmodel.NewEffectScopeConfig(8, 8),
			false,
			repo,
			nil,
		)
		defer endOfStateHandler()

		// 1. Get sink channel from SourceStatePayload
		chVal, err := state.Effect(ctx, state.Source{})
		require.NoError(t, err)

		sink, ok := chVal.(chan state.TimeBoundedPayload)
		require.True(t, ok, "expected sink channel from SourceStatePayload")

		// 2. Send a StoreStatePayload
		key := "test-key"
		newVal := "new-value"
		_, err = state.Effect(ctx, state.InsertPayloadOf(key, newVal))
		require.NoError(t, err)

		// 3. Check the sink channel for the StoreStatePayload
		select {
		case payload := <-sink:
			storePayload, ok := payload.Payload.(state.InsertIfAbsent[string, string])
			require.True(t, ok)
			assert.Equal(t, storePayload.Key, key)
			assert.Equal(t, storePayload.New, newVal)
		case <-time.After(1 * time.Second):
			t.Fatal("expected payload not received on sink channel")
		}

		// 4. Send a CompareAndDeleteStatePayload
		oldVal := "new-value"
		_, err = state.Effect(ctx, state.CADPayloadOf(key, oldVal))
		require.NoError(t, err)

		// 5. Check the sink channel for the CompareAndDeleteStatePayload
		select {
		case payload := <-sink:
			storePayload, ok := payload.Payload.(state.CompareAndDelete[string, string])
			require.True(t, ok)
			assert.Equal(t, storePayload.Key, key)
			assert.Equal(t, storePayload.Old, oldVal)
		case <-time.After(1 * time.Second):
			t.Fatal("expected payload not received on sink channel")
		}
	}

	for _, repo := range []state.StateRepo{newTestCasRepo()} {
		testFn(repo)
	}
}

func TestStore_Delegation(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(repo state.StateRepo) {
		// Tier 2 - has key x=1
		ctx2, endOfTier2 := state.WithEffectHandler(
			ctx,
			effectmodel.NewEffectScopeConfig(2, 2),
			false,
			newTestCasRepo(),
			map[string]int{},
		)
		defer endOfTier2()

		// Tier 1 - has key x=1
		ctx1, endOfTier1 := state.WithEffectHandler(
			ctx2,
			effectmodel.NewEffectScopeConfig(2, 2),
			false,
			newTestCasRepo(),
			map[string]int{},
		)
		defer endOfTier1()

		// Tier 0 - has key x=1
		ctx0, endOfTier0 := state.WithEffectHandler(
			ctx1,
			effectmodel.NewEffectScopeConfig(2, 2),
			true,
			repo,
			map[string]int{},
		)
		defer endOfTier0()

		// Store should succeed in tier 0
		_, err := state.Effect(ctx0, state.InsertPayloadOf("x", 2))
		assert.NoError(t, err, "Store delegation failed")

		// Confirm that new value is set
		val, err := state.Effect(ctx0, state.LoadPayloadOf("x"))
		assert.NoError(t, err, "Load failed")
		assert.Equal(t, val.(int), 2, "Expected x=2")

		// Confirm that new value is set
		val, err = state.Effect(ctx1, state.LoadPayloadOf("x"))
		assert.NoError(t, err, "Load failed")
		assert.Equal(t, val.(int), 2, "Expected x=2")

		// Confirm that prev value is set
		_, err = state.Effect(ctx2, state.LoadPayloadOf("x"))
		assert.ErrorIs(t, err, state.ErrNoSuchKey, "should be no such key")
	}

	for _, repo := range []state.StateRepo{newTestCasRepo()} {
		testFn(repo)
	}

}

func TestCompareAndSwap_Delegation(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(repo state.StateRepo) {
		// Tier 2 - has key x=1
		ctx2, endOfTier2 := state.WithEffectHandler(
			ctx,
			effectmodel.NewEffectScopeConfig(2, 2),
			false,
			newTestCasRepo(),
			map[string]int{
				"x": 1,
			},
		)
		defer endOfTier2()

		// Tier 1 - has key x=1
		ctx1, endOfTier1 := state.WithEffectHandler(
			ctx2,
			effectmodel.NewEffectScopeConfig(2, 2),
			false,
			newTestCasRepo(),
			map[string]int{
				"x": 1,
			},
		)
		defer endOfTier1()

		// Tier 0 - has key x=1
		ctx0, endOfTier0 := state.WithEffectHandler(
			ctx1,
			effectmodel.NewEffectScopeConfig(2, 2),
			true,
			repo,
			map[string]int{
				"x": 1,
			},
		)
		defer endOfTier0()

		// CAS should succeed in tier 0
		ok, err := state.Effect(ctx0, state.CASPayloadOf("x", 1, 2))
		assert.NoError(t, err, "CAS delegation failed")
		assert.True(t, ok.(bool), "CAS delegation returned false, expected true")

		// Confirm that new value is set
		val, err := state.Effect(ctx0, state.LoadPayloadOf("x"))
		assert.NoError(t, err, "Load failed")
		assert.Equal(t, val.(int), 2, "Expected x=2")

		// Confirm that new value is set
		val, err = state.Effect(ctx1, state.LoadPayloadOf("x"))
		assert.NoError(t, err, "Load failed")
		assert.Equal(t, val.(int), 2, "Expected x=2")

		// Confirm that prev value is set
		val, err = state.Effect(ctx2, state.LoadPayloadOf("x"))
		assert.NoError(t, err, "Load failed")
		assert.Equal(t, val.(int), 1, "Expected x=1")
	}

	for _, repo := range []state.StateRepo{newTestCasRepo()} {
		testFn(repo)
	}

}

func TestCompareAndDelete_Delegation(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	testFn := func(repo state.StateRepo) {
		// Tier 2 - has key y=99
		ctx2, endOfTier2 := state.WithEffectHandler(
			ctx,
			effectmodel.NewEffectScopeConfig(2, 2),
			false,
			newTestCasRepo(),
			map[string]int{
				"y": 98,
			},
		)
		defer endOfTier2()

		// Tier 1 - has key y=99
		ctx1, endOfTier1 := state.WithEffectHandler(
			ctx2,
			effectmodel.NewEffectScopeConfig(2, 2),
			false,
			newTestCasRepo(),
			map[string]int{
				"y": 99,
			},
		)
		defer endOfTier1()

		// Tier 0 - has key y=99
		ctx0, endOfTier0 := state.WithEffectHandler(
			ctx1,
			effectmodel.NewEffectScopeConfig(2, 2),
			true,
			repo,
			map[string]int{
				"y": 99,
			},
		)
		defer endOfTier0()

		// CAD should succeed in tier 0
		ok, err := state.Effect(ctx0, state.CADPayloadOf("y", 99))
		assert.NoError(t, err, "CAD delegation failed")
		assert.True(t, ok.(bool), "CAD delegation returned false, expected true")

		// Confirm that prev value is set
		val, err := state.Effect(ctx2, state.LoadPayloadOf("y"))
		assert.NoError(t, err, "Load failed")
		assert.Equal(t, val.(int), 98, "Expected y=99")
	}

	for _, repo := range []state.StateRepo{newTestCasRepo()} {
		testFn(repo)
	}

}
