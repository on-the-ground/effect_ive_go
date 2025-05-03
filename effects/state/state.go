package state

import (
	"context"
	"errors"
	"fmt"

	"github.com/on-the-ground/effect_ive_go/effects"
	"github.com/on-the-ground/effect_ive_go/effects/concurrency"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
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
func WithEffectHandler[K comparable, V comparable](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	delegation bool,
	stateRepo StateRepo,
	initMap map[K]V,
) (context.Context, func() context.Context) {
	ctx, endOfConcurrency := concurrency.WithEffectHandler(ctx, config.BufferSize)
	sink := make(chan TimeBoundedPayload, 2*config.NumWorkers)
	stateHandler := &stateHandler[K, V]{
		stateRepo:  stateRepo,
		sink:       sink,
		delegation: delegation,
	}
	for k, v := range initMap {
		stateHandler.insertIfAbsent(k, v)
	}
	return effects.WithResumablePartitionableEffectHandler(
		ctx,
		config,
		effectmodel.EffectState,
		stateHandler.handle,
		func() {
			endOfConcurrency()
			close(sink)
		},
	)
}

// Effect performs a state operation (get, set, delete) using the EffectState handler.
func Effect(ctx context.Context, payload Payload) (val any, err error) {
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
	return Effect(upperCtx, payload)
}

// stateHandler defines the in-memory state store logic.
// It supports safe concurrent access and fallback to upstream handler if key is missing.
type stateHandler[K comparable, V comparable] struct {
	stateRepo  StateRepo
	sink       chan TimeBoundedPayload
	delegation bool
}

func (sH stateHandler[K, V]) compareAndSwap(k K, old, new V) bool {
	return matchRepo(sH.stateRepo,
		func(repo casRepo) bool {
			return repo.CompareAndSwap(k, old, new)
		},
		func(repo setRepo) bool {
			if cur, ok := repo.Get(k); !ok {
				return false
			} else if cur != old {
				return false
			} else {
				repo.Set(k, new)
				return true
			}
		},
	)
}

func (sH stateHandler[K, V]) compareAndDelete(k K, old V) bool {
	return matchRepo(sH.stateRepo,
		func(repo casRepo) bool {
			return repo.CompareAndDelete(k, old)
		},
		func(repo setRepo) bool {
			if cur, ok := repo.Get(k); !ok {
				return false
			} else if cur != old {
				return false
			} else {
				repo.Delete(k)
				return true
			}
		},
	)
}

func (sH stateHandler[K, V]) insertIfAbsent(k K, v V) {
	matchRepo(sH.stateRepo,
		func(repo casRepo) any {
			repo.InsertIfAbsent(k, v)
			return struct{}{}
		},
		func(repo setRepo) any {
			repo.Set(k, v)
			return struct{}{}
		},
	)
}

func (sH stateHandler[K, V]) load(k K) (V, bool) {
	type res struct {
		v  V
		ok bool
	}
	ret := matchRepo(sH.stateRepo,
		func(repo casRepo) res {
			v, ok := repo.Load(k)
			if !ok {
				return res{v: *new(V), ok: false}
			}
			return res{v: v.(V), ok: ok}
		},
		func(repo setRepo) res {
			v, ok := repo.Get(k)
			if !ok {
				return res{v: *new(V), ok: false}
			}
			return res{v: v.(V), ok: ok}
		},
	)
	return ret.v, ret.ok
}

// handle routes the given payload to the appropriate state operation logic.
func (sH stateHandler[K, V]) handle(ctx context.Context, payload Payload) (res any, err error) {
	switch payload := payload.(type) {

	case CompareAndSwap[K, V]:
		if payload.Old == payload.New {
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
		if swapped := sH.compareAndSwap(payload.Key, payload.Old, payload.New); !swapped {
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
		if deleted := sH.compareAndDelete(payload.Key, payload.Old); !deleted {
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
				delegateStateEffect(ctx, payload)
			}()
		}
		sH.insertIfAbsent(payload.Key, payload.New)
		payloadWithTimeSpan := statePayloadWithNow(payload)
		concurrency.Effect(ctx, func(ctx context.Context) {
			select {
			case <-ctx.Done():
			case sH.sink <- payloadWithTimeSpan:
			default:
			}
		})
		return nil, nil

	case load[K]:
		v, ok := sH.load(payload.Key)
		if ok {
			return v, nil
		}
		if v, err := delegateStateEffect(ctx, payload); err != nil {
			return nil, ErrNoSuchKey
		} else {
			return v, nil
		}

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
