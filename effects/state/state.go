package state

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/on-the-ground/effect_ive_go/effects"
	"github.com/on-the-ground/effect_ive_go/effects/concurrency"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

var _ StatePayload = SourceStatePayload{}

// SourceStatePayload is a special payload type for the state effect handler.
type SourceStatePayload struct{}

func (SourceStatePayload) PartitionKey() string         { return "" }
func (SourceStatePayload) sealedInterfaceStatePayload() {}

var _ StatePayload = LoadStatePayload{}

// LoadStatePayload is the payload type for retrieving a value from the state.
type LoadStatePayload struct {
	Key any // should be comparable
}

// PartitionKey returns the partition key for routing this payload.
func (p LoadStatePayload) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}

// sealedInterfaceStatePayload prevents external packages from implementing statePayload.
func (p LoadStatePayload) sealedInterfaceStatePayload() {}

var _ StatePayload = CompareAndDeleteStatePayload{}

// CompareAndDeleteStatePayload is the payload type for deleting a key from the state.
type CompareAndDeleteStatePayload struct {
	Key any // should be comparable
	Old any // should be comparable
}

func (p CompareAndDeleteStatePayload) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}
func (p CompareAndDeleteStatePayload) sealedInterfaceStatePayload() {}

var _ StatePayload = StoreStatePayload{}

// StoreStatePayload is the payload type for deleting a key from the state.
type StoreStatePayload struct {
	Key any // should be comparable
	New any // should be comparable
}

func (p StoreStatePayload) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}
func (p StoreStatePayload) sealedInterfaceStatePayload() {}

// CompareAndSwapStatePayload is the payload type for inserting or updating a key-value pair.
type CompareAndSwapStatePayload struct {
	Key any // should be comparable
	New any // should be comparable
	Old any // should be comparable
}

func (p CompareAndSwapStatePayload) PartitionKey() string         { return fmt.Sprintf("%v", p.Key) }
func (p CompareAndSwapStatePayload) sealedInterfaceStatePayload() {}

// StatePayload is a sealed interface for state operations.
// Only predefined payload types (Set, Get, Delete) can implement this interface.
type StatePayload interface {
	PartitionKey() string
	sealedInterfaceStatePayload()
}

// WithStateEffectHandler registers a resumable, partitionable effect handler for managing key-value state.
// It stores the internal state in a memory-safe sync.Map and supports sharded processing.
// The handler is resumable and partitionable, meaning it can be resumed after a failure
// and can handle multiple partitions concurrently.
// The handler is registered in the context and can be used to perform state operations.
// The handler is closed when the context is canceled or when the teardown function is called.
// The teardown function should be called when the effect handler is no longer needed.
// If the teardown function is called early, the effect handler will be closed.
// The context returned by the teardown function should be used for further operations.
func WithStateEffectHandler(
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	delegation bool,
	initMap map[string]any,
) (context.Context, func() context.Context) {
	ctx, endOfConcurrency := concurrency.WithConcurrencyEffectHandler(ctx, config.BufferSize)
	sink := make(chan TimeBoundedStatePayload, 2*config.NumWorkers)
	stateHandler := &stateHandler{
		stateMap:   &sync.Map{},
		sink:       sink,
		delegation: delegation,
	}
	for k, v := range initMap {
		stateHandler.stateMap.Store(k, v)
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

// StateEffect performs a state operation (get, set, delete) using the EffectState handler.
func StateEffect(ctx context.Context, payload StatePayload) (val any, err error) {
	resultCh := effects.PerformResumableEffect[StatePayload, any](ctx, effectmodel.EffectState, payload)
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
func delegateStateEffect(upperCtx context.Context, payload StatePayload) (res any, err error) {
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
	return StateEffect(upperCtx, payload)
}

// stateHandler defines the in-memory state store logic.
// It supports safe concurrent access and fallback to upstream handler if key is missing.
type stateHandler struct {
	stateMap   *sync.Map
	sink       chan TimeBoundedStatePayload
	delegation bool
}

// handle routes the given payload to the appropriate state operation logic.
func (sH stateHandler) handle(ctx context.Context, payload StatePayload) (res any, err error) {
	switch payload := payload.(type) {

	case CompareAndSwapStatePayload:
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
		if swapped := sH.stateMap.CompareAndSwap(payload.Key, payload.Old, payload.New); !swapped {
			res = false
			err = nil
			return
		}
		payloadWithTimeSpan := statePayloadWithNow(payload)

		concurrency.ConcurrencyEffect(ctx, func(ctx context.Context) {
			select {
			case <-ctx.Done():
			case sH.sink <- payloadWithTimeSpan:
			default:
			}
		})

		res = true
		err = nil
		return

	case CompareAndDeleteStatePayload:
		if sH.delegation {
			defer func() {
				dres, _ := delegateStateEffect(ctx, payload)
				res = res.(bool) || dres.(bool)
			}()
		}
		if deleted := sH.stateMap.CompareAndDelete(payload.Key, payload.Old); !deleted {
			res = false
			err = nil
			return
		}
		payloadWithTimeSpan := statePayloadWithNow(payload)

		concurrency.ConcurrencyEffect(ctx, func(ctx context.Context) {
			select {
			case <-ctx.Done():
			case sH.sink <- payloadWithTimeSpan:
			default:
			}
		})

		res = true
		err = nil
		return

	case StoreStatePayload:
		if sH.delegation {
			defer func() {
				delegateStateEffect(ctx, payload)
			}()
		}
		sH.stateMap.Store(payload.Key, payload.New)
		payloadWithTimeSpan := statePayloadWithNow(payload)
		concurrency.ConcurrencyEffect(ctx, func(ctx context.Context) {
			select {
			case <-ctx.Done():
			case sH.sink <- payloadWithTimeSpan:
			default:
			}
		})
		return nil, nil

	case LoadStatePayload:
		v, ok := sH.stateMap.Load(payload.Key)
		if ok {
			return v, nil
		}
		if v, err := delegateStateEffect(ctx, payload); err != nil {
			return nil, ErrNoSuchKey
		} else {
			return v, nil
		}

	case SourceStatePayload:
		return sH.sink, nil

	default:
		// This should never happen because we are using a sealed interface to prevent adding new types.
		// So we need to panic to avoid silent failures.
		// This is a bug in the code.
		panic(fmt.Errorf("invalid state operation type: %T", payload))
	}
}

// TimeBoundedStatePayload is a wrapper for StatePayload with a time span.
type TimeBoundedStatePayload struct {
	StatePayload
	effects.TimeSpan
}

func statePayloadWithNow(payload StatePayload) TimeBoundedStatePayload {
	return TimeBoundedStatePayload{StatePayload: payload, TimeSpan: effects.Now()}
}
