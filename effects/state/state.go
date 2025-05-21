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
	"github.com/on-the-ground/effect_ive_go/effects/stream"
	"github.com/on-the-ground/effect_ive_go/shared"
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
	concurrencyHandlerId := concurrency.MustHaveEffectHandler(ctx)

	sink := make(chan TimeBoundedPayload, 2*numWorkers)
	stateHandler := &stateHandler[K, V]{
		stateStore: stateStore,
		timers:     make(map[K]*time.Timer),
		sink:       sink,
		delegation: delegation,
	}
	for k, v := range initMap {
		stateHandler.insertIfAbsent(k, v)
	}

	ctx, endOfStateHandler := effects.WithResumablePartitionableEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(bufferSize, numWorkers),
		effectmodel.EffectState,
		stateHandler.handle,
		func() {
			close(sink)
		},
	)
	log.Effect(ctx, log.LogInfo, "concurrency handler of this state handler: ", map[string]interface{}{
		"streamHander":       MustHaveEffectHandler(ctx),
		"concurrencyHandler": concurrencyHandlerId,
	})

	return ctx, endOfStateHandler
}

// MustHaveEffectHandler ensures that the state effect handler is installed.
// It panics if the effect handler is not installed.
func MustHaveEffectHandler(ctx context.Context) string {
	return effects.ResumableEffectHandlerId[Payload, any](ctx, effectmodel.EffectState)
}

func effectSource(ctx context.Context) (chan TimeBoundedPayload, error) {
	return helper.GetTypedValueOf[chan TimeBoundedPayload](func() (any, error) {
		return effect(ctx, Source{})
	})
}

// EventSourcingEffect subscribes to the effect source and sends the received payloads to the provided sink and dropped channels.
// It logs an error if the effect source is not found.
// WARNING: Stream effect handler must be created before calling this function.
func EventSourcingEffect(
	ctx context.Context,
	sink chan<- TimeBoundedPayload,
	dropped chan<- TimeBoundedPayload,
) {
	src, err := effectSource(ctx)
	if err != nil {
		log.Effect(ctx, log.LogError, "fail to subscribe source, effect source not found", map[string]interface{}{
			"err": err,
		})
		return
	}
	stream.SubscribeEffect(ctx, src, sink, dropped)
}

// LoadOverlaidEffect loads a value from the state store using the provided key.
// It first checks the local state store and then delegates to the upper handler if not found.
// If the value is found in the upper handler, it is inserted into the local state store.
// It returns an error if the key is not found in both stores.
func LoadOverlaidEffect[K comparable, V ComparableEquatable](ctx context.Context, key K) (val V, err error) {
	return helper.GetTypedValueOf[V](func() (any, error) {
		return effect(ctx, load[K]{Key: key})
	})
}

// LoadEffect loads a value from the state store using the provided key.
// It only checks the local state store and does not delegate to the upper handler.
// It returns an error if the key is not found.
func LoadEffect[K comparable, V ComparableEquatable](ctx context.Context, key K) (val V, err error) {
	return helper.GetTypedValueOf[V](func() (any, error) {
		return effect(ctx, loadWoDelegation[K]{Key: key})
	})
}

// LoadEffects loads multiple values from the state store using the provided prefix.
// It returns a map of key-value pairs found in the state store.
// It does not delegate to the upper handler and returns an error if the prefix is not found.
// Note: This function is not implemented yet.
func LoadEffects[V ComparableEquatable](ctx context.Context, prefix string) (val map[string]V, err error) {
	// todo https://github.com/on-the-ground/effect_ive_go/issues/46
	panic("not implemented")
}

// InsertIfAbsentEffect inserts a new value into the state store if the key is not already present.
// It returns true if the value was inserted, false if it already exists, and an error if the operation fails.
// It delegates to the upper handler if the delegation flag is set on the state handler.
func InsertIfAbsentEffect[K comparable, V ComparableEquatable](
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

// InsertIfAbsentWithTTLEffect inserts a new value into the state store with a TTL if the key is not already present.
// It returns true if the value was inserted, false if it already exists, and an error if the operation fails.
// It sets a timer to delete the key after the specified TTL duration.
// It delegates to the upper handler if the delegation flag is set on the state handler without setting the timer.
func InsertIfAbsentWithTTLEffect[K comparable, V ComparableEquatable](
	ctx context.Context,
	key K,
	new V,
	ttl time.Duration,
) (inserted bool, err error) {

	inserted, err = InsertIfAbsentEffect(ctx, key, new)
	if inserted {
		effect(ctx, setTTL[K]{
			Key: key,
			TTL: ttl,
		})
	}
	return
}

// ResetTTLEffect resets the TTL of the specified key in the state store.
// It returns true if the TTL was reset, false if the key was not found, and an error if the operation fails.
// It sets a new timer to delete the key after the specified TTL duration.
func ResetTTLEffect[K comparable](
	ctx context.Context,
	key K,
	ttl time.Duration,
) (reset bool, err error) {
	return helper.GetTypedValueOf[bool](func() (any, error) {
		return effect(ctx, resetTTL[K]{
			Key: key,
			TTL: ttl,
		})
	})
}

// CompareAndDeleteEffect compares the current value of the specified key with the provided old value.
// If they match, it deletes the key from the state store and returns true.
// If they do not match, it returns false and an error if the operation fails.
// It delegates to the upper handler if the delegation flag is set on the state handler.
func CompareAndDeleteEffect[K comparable, V ComparableEquatable](
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

// CompareAndSwapEffect compares the current value of the specified key with the provided old value.
// If they match, it updates the key with the new value and returns true.
// If they do not match, it returns false and an error if the operation fails.
// It delegates to the upper handler if the delegation flag is set on the state handler.
func CompareAndSwapEffect[K comparable, V ComparableEquatable](
	ctx context.Context,
	key K,
	old, new V,
) (swapped bool, err error) {
	return helper.GetTypedValueOf[bool](func() (any, error) {
		return effect(ctx, compareAndSwap[K, V]{
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
	timers     map[K]*time.Timer // todo https://github.com/on-the-ground/effect_ive_go/issues/45
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
			} else if !Equals(cur, old) {
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
			} else if !Equals(cur, old) {
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

	case setTTL[K]:
		key := payload.Key
		ttl := payload.TTL
		timer := time.NewTimer(ttl)

		// If the timer is done, we need to delete the key from the state store.
		concurrency.Effect(ctx, func(svCtx context.Context) {
			defer log.Effect(ctx, log.LogDebug, "Expiry handler finished", map[string]interface{}{
				"key": key,
				"ttl": ttl,
			})
			defer timer.Stop()

			log.Effect(ctx, log.LogDebug, "Expiry handler started", map[string]interface{}{
				"key": key,
				"ttl": ttl,
			})

			select {
			case <-svCtx.Done():
			case <-timer.C:
				old, err := helper.GetTypedValueOf[V](func() (any, error) {
					return sH.handle(ctx, load[K]{Key: key})
				})
				log.Effect(ctx, log.LogDebug, "loaded value before cas", map[string]interface{}{
					"key":   key,
					"value": fmt.Sprintf("%#v", old),
				})
				if err != nil {
					log.Effect(ctx, log.LogError, "fail to load value from parent handler", map[string]interface{}{
						"key": key,
						"err": err,
					})
				}

				deleted, err := helper.GetTypedValueOf[bool](func() (any, error) {
					return sH.handle(ctx, CompareAndDelete[K, V]{Key: key, Old: old})
				})
				if err != nil {
					log.Effect(ctx, log.LogError, "fail to delete old value of parent handler", map[string]interface{}{
						"key": key,
						"err": err,
					})
				}
				if !deleted {
					log.Effect(ctx, log.LogInfo, "cas failed, will retry", map[string]interface{}{
						"key":    key,
						"old":    old,
						"status": "cas_mismatch",
					})
				}
				log.Effect(ctx, log.LogInfo, "deleted value of parent handler", map[string]interface{}{
					"key":    key,
					"old":    old,
					"status": "deleted",
				})
			}
		})
		sH.timers[key] = timer
		res = true
		err = nil
		return

	case resetTTL[K]:
		timer, ok := sH.timers[payload.Key]
		if !ok {
			return nil, fmt.Errorf("timer not found for key: %v", payload.Key)
		}

		if !timer.Stop() {
			// drain timer.C to avoid race
			select {
			case <-timer.C:
			default:
			}
		}

		timer.Reset(payload.TTL)
		return true, nil

	case compareAndSwap[K, V]:
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

	case load[K]:
		v, ok, err := sH.load(payload.Key)
		if err != nil {
			return *new(V), err
		}
		if ok {
			return v, nil
		}
		if v, err := delegateStateEffect(ctx, payload); err != nil {
			return nil, ErrNoSuchKey
		} else if sH.delegation {
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
	return
}

// TimeBoundedPayload is a wrapper for StatePayload with a time span.
type TimeBoundedPayload struct {
	Payload
	shared.TimeSpan
}

func statePayloadWithNow(payload Payload) TimeBoundedPayload {
	return TimeBoundedPayload{Payload: payload, TimeSpan: shared.Now()}
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

	if swapped, err := CompareAndSwapEffect(ctx, payload.Key, old, payload.New); err != nil {
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
