package state

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects"
	"github.com/on-the-ground/effect_ive_go/effects/concurrency"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/shared/helper"
)

// WithEffectHandler registers a resumable, partitionable effect handler for managing key-value state.
// It stores the internal state in a memory-safe sync.Map and supports sharded processing.
// The handler is resumable and partitionable, meaning it can be resumed after a failure
// and can handle multiple partitions concurrently.
// The handler is registered in the context and can be used to perform state operations.
// The handler is closed when the context is canceled or when the teardown function is called.
// The teardown function should be called when the effect handler is no longer needed.
// If the teardown function is called early, the effect handler will be closed.
// The context returned by the teardown function should be used for further operations.
func WithEffectHandler[K comparable, V ComparableEquatable](
	ctx context.Context,
	bufferSize, numWorkers int,
	delegation bool,
	stateStore StateStore,
	initMap map[K]V,
) (context.Context, func() context.Context) {
	ctx, endOfConcurrency := concurrency.WithEffectHandler(ctx, bufferSize)
	sink := make(chan TimeBoundedPayload, 2*numWorkers)
	stateHandler := &stateHandler[K, V]{
		stateStore: stateStore,
		sink:       sink,
		delegation: delegation,
	}
	for k, v := range initMap {
		stateHandler.insertIfAbsent(k, v)
	}
	return effects.WithResumablePartitionableEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(bufferSize, numWorkers),
		effectmodel.EffectState,
		stateHandler.handle,
		func() {
			endOfConcurrency()
			close(sink)
		},
	)
}

func EffectSource(ctx context.Context) (chan TimeBoundedPayload, error) {
	return helper.GetTypedValueOf[chan TimeBoundedPayload](func() (any, error) {
		return effect(ctx, Source{})
	})
}

func EffectLoad[K comparable, V ComparableEquatable](
	ctx context.Context,
	key K,
) (val V, err error) {
	return helper.GetTypedValueOf[V](func() (any, error) {
		return effect(ctx, Load[K]{Key: key})
	})
}

func EffectInsertIfAbsent[K comparable, V ComparableEquatable](
	ctx context.Context,
	key K,
	new V,
) (inserted bool, err error) {
	return helper.GetTypedValueOf[bool](func() (any, error) {
		return effect(ctx, InsertIfAbsent[K, V]{
			Key: key,
			New: new,
		})
	})
}

func EffectCompareAndDelete[K comparable, V ComparableEquatable](
	ctx context.Context,
	key K,
	old V,
) (deleted bool, err error) {
	return helper.GetTypedValueOf[bool](func() (any, error) {
		return effect(ctx, CompareAndDelete[K, V]{
			Key: key,
			Old: old,
		})
	})
}

func EffectCompareAndSwap[K comparable, V ComparableEquatable](
	ctx context.Context,
	key K,
	old, new V,
) (swapped bool, err error) {
	return helper.GetTypedValueOf[bool](func() (any, error) {
		return effect(ctx, CompareAndSwap[K, V]{
			Key: key,
			Old: old,
			New: new,
		})
	})
}

// effect performs a state operation (get, set, delete) using the EffectState handler.
func effect(ctx context.Context, payload Payload) (val any, err error) {
	resultCh := effects.PerformResumableEffect[Payload, any](ctx, effectmodel.EffectState, payload)
	select {
	case res, ok := <-resultCh:
		if ok {
			val = res.Value
			err = res.Err
			return
		}
	case <-ctx.Done():
	}
	err = ctx.Err()
	return
}

// ErrNoSuchKey is an error indicating that the key was not found in any state handlers.
var ErrNoSuchKey = fmt.Errorf("key not found")

// delegateStateEffect is an internal helper for performing the state effect directly.
func delegateStateEffect(upperCtx context.Context, payload Payload) (res any, err error) {
	defer func() {
		if r := recover(); r != nil {
			if r, ok := r.(error); ok && errors.Is(r, effectmodel.ErrNoEffectHandler) {
				// Handle panic and return a nil value with an error
				// indicating that the effect handler is not available to delegate.
				res = nil
				err = r
			} else {
				panic(r) // re-raise the panic if it's not the expected error
			}
		}
	}()

	// Delegate the effect to the upper handler
	return effect(upperCtx, payload)
}

// stateHandler defines the in-memory state store logic.
// It supports safe concurrent access and fallback to upstream handler if key is missing.
type stateHandler[K comparable, V ComparableEquatable] struct {
	stateStore StateStore
	sink       chan TimeBoundedPayload
	delegation bool
}

func (sH stateHandler[K, V]) compareAndSwap(k K, old, new V) (bool, error) {
	return matchStore(sH.stateStore,
		func(store casStore[K]) (bool, error) {
			return store.CompareAndSwap(k, old, new)
		},
		func(store setStore[K]) (bool, error) {
			if cur, ok, err := store.Get(k); !ok || err != nil {
				return false, err
			} else if Equals(cur, old) {
				return false, nil
			} else {
				store.Set(k, new)
				return true, nil
			}
		},
	)
}

func (sH stateHandler[K, V]) compareAndDelete(k K, old V) (bool, error) {
	return matchStore(sH.stateStore,
		func(store casStore[K]) (bool, error) {
			return store.CompareAndDelete(k, old)
		},
		func(store setStore[K]) (bool, error) {
			if cur, ok, err := store.Get(k); !ok || err != nil {
				return false, err
			} else if Equals(cur, old) {
				return false, nil
			} else {
				store.Delete(k)
				return true, nil
			}
		},
	)
}

func (sH stateHandler[K, V]) insertIfAbsent(k K, v V) (bool, error) {
	return matchStore(sH.stateStore,
		func(store casStore[K]) (bool, error) {
			return store.InsertIfAbsent(k, v)
		},
		func(store setStore[K]) (bool, error) {
			if _, ok, err := store.Get(k); err != nil {
				return false, err
			} else if ok {
				return false, nil
			}
			store.Set(k, v)
			return true, nil
		},
	)
}

func (sH stateHandler[K, V]) load(k K) (V, bool, error) {
	type res struct {
		v  V
		ok bool
	}
	ret, err := matchStore(sH.stateStore,
		func(store casStore[K]) (res, error) {
			v, ok, err := store.Load(k)
			if err != nil {
				return *new(res), err
			}
			if !ok {
				return res{v: *new(V), ok: false}, nil
			}
			return res{v: v.(V), ok: ok}, nil
		},
		func(store setStore[K]) (res, error) {
			v, ok, err := store.Get(k)
			if err != nil {
				return *new(res), err
			}
			if !ok {
				return res{v: *new(V), ok: false}, nil
			}
			return res{v: v.(V), ok: ok}, nil
		},
	)
	return ret.v, ret.ok, err
}

// handle routes the given payload to the appropriate state operation logic.
func (sH stateHandler[K, V]) handle(ctx context.Context, payload Payload) (res any, err error) {
	switch payload := payload.(type) {

	case CompareAndSwap[K, V]:
		if Equals(payload.Old, payload.New) {
			res = true
			err = nil
			return
		}
		if sH.delegation {
			defer func() {
				dres, _ := delegateStateEffect(ctx, payload)
				res = res.(bool) || dres.(bool)
			}()
		}
		var swapped bool
		if swapped, err = sH.compareAndSwap(payload.Key, payload.Old, payload.New); err != nil {
			res = false
			return
		} else if !swapped {
			res = false
			err = nil
			return
		}
		payloadWithTimeSpan := statePayloadWithNow(payload)

		concurrency.Effect(ctx, func(ctx context.Context) {
			select {
			case <-ctx.Done():
			case sH.sink <- payloadWithTimeSpan:
			default:
			}
		})

		res = true
		err = nil
		return

	case CompareAndDelete[K, V]:
		if sH.delegation {
			defer func() {
				dres, _ := delegateStateEffect(ctx, payload)
				res = res.(bool) || dres.(bool)
			}()
		}
		var deleted bool
		if deleted, err = sH.compareAndDelete(payload.Key, payload.Old); err != nil {
			res = false
			return
		} else if !deleted {
			res = false
			err = nil
			return
		}
		payloadWithTimeSpan := statePayloadWithNow(payload)

		concurrency.Effect(ctx, func(ctx context.Context) {
			select {
			case <-ctx.Done():
			case sH.sink <- payloadWithTimeSpan:
			default:
			}
		})

		res = true
		err = nil
		return

	case InsertIfAbsent[K, V]:
		if sH.delegation {
			defer func() {
				if inserted, err := helper.GetTypedValueOf[bool](func() (any, error) {
					return delegateStateEffect(ctx, payload)
				}); err != nil {
					log.Effect(ctx, log.LogError, "fail to delegate insertion", map[string]interface{}{
						"payload": payload,
						"err":     err,
					})
				} else if alreadyExist := !inserted; alreadyExist {
					helper.Retry(5, func() error {
						err := tryToUpdate(ctx, payload)
						if err != nil {
							time.Sleep(10 * time.Millisecond)
						}
						return err
					})
				}
			}()
		}
		res, err = sH.insertIfAbsent(payload.Key, payload.New)
		payloadWithTimeSpan := statePayloadWithNow(payload)
		concurrency.Effect(ctx, func(ctx context.Context) {
			select {
			case <-ctx.Done():
			case sH.sink <- payloadWithTimeSpan:
			default:
			}
		})
		return

	case Load[K]:
		v, ok, err := sH.load(payload.Key)
		if err != nil {
			return *new(V), err
		}
		if ok {
			return v, nil
		}
		if v, err := delegateStateEffect(ctx, payload); err != nil {
			return nil, ErrNoSuchKey
		} else {
			sH.insertIfAbsent(payload.Key, v.(V))
			return v, nil
		}

	case loadWoDelegation[K]:
		v, ok, err := sH.load(payload.Key)
		if err != nil {
			return *new(V), err
		}
		if ok {
			return v, nil
		}
		return nil, ErrNoSuchKey

	case Source:
		return sH.sink, nil

	default:
		// This should never happen because we are using a sealed interface to prevent adding new types.
		// So we need to panic to avoid silent failures.
		// This is a bug in the code.
		panic(fmt.Errorf("invalid state operation type: %T", payload))
	}
}

// TimeBoundedPayload is a wrapper for StatePayload with a time span.
type TimeBoundedPayload struct {
	Payload
	effects.TimeSpan
}

func statePayloadWithNow(payload Payload) TimeBoundedPayload {
	return TimeBoundedPayload{Payload: payload, TimeSpan: effects.Now()}
}

func tryToUpdate[K comparable, V ComparableEquatable](ctx context.Context, payload InsertIfAbsent[K, V]) error {
	raw, err := effect(ctx, loadWoDelegation[K]{Key: payload.Key})
	if err != nil {
		log.Effect(ctx, log.LogError, "fail to load value from parent handler", map[string]interface{}{
			"key": payload.Key,
			"err": err,
		})
		return err
	}
	old := raw.(V)
	if Equals(old, payload.New) {
		return nil
	}
	log.Effect(ctx, log.LogInfo, "the value of parent handler is outdated", map[string]interface{}{
		"key": payload.Key,
		"old": old,
		"new": payload.New,
	})

	if swapped, err := EffectCompareAndSwap(ctx, payload.Key, old, payload.New); err != nil {
		log.Effect(ctx, log.LogError, "fail to update old value of parent handler", map[string]interface{}{
			"key": payload.Key,
			"err": err,
		})
		return err
	} else if !swapped {
		log.Effect(ctx, log.LogInfo, "the old value has changed, retry cas", map[string]interface{}{
			"key": payload.Key,
			"err": err,
		})
		return errors.New("fail to cas")
	}
	return nil
}
