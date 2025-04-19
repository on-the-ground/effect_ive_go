package effects

import (
	"context"
	"errors"
	"fmt"
	"sync"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

var _ StatePayload = SourceStatePayload{}

// SourceStatePayload is a special payload type for the state effect handler.
type SourceStatePayload struct{}

func (SourceStatePayload) PartitionKey() string         { return "" }
func (SourceStatePayload) sealedInterfaceStatePayload() {}

var _ StatePayload = GetStatePayload{}

// GetStatePayload is the payload type for retrieving a value from the state.
type GetStatePayload struct {
	Key string
}

// PartitionKey returns the partition key for routing this payload.
func (p GetStatePayload) PartitionKey() string { return p.Key }

// sealedInterfaceStatePayload prevents external packages from implementing statePayload.
func (p GetStatePayload) sealedInterfaceStatePayload() {}

var _ StatePayload = DeleteStatePayload{}

// DeleteStatePayload is the payload type for deleting a key from the state.
type DeleteStatePayload struct {
	Key string
}

func (p DeleteStatePayload) PartitionKey() string         { return p.Key }
func (p DeleteStatePayload) sealedInterfaceStatePayload() {}

type Equatable interface {
	Equals(i any) bool
}

var _ StatePayload = SetStatePayload{}

// SetStatePayload is the payload type for inserting or updating a key-value pair.
type SetStatePayload struct {
	Key   string
	Value Equatable
}

func (p SetStatePayload) PartitionKey() string         { return p.Key }
func (p SetStatePayload) sealedInterfaceStatePayload() {}

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
	initMap map[string]any,
) (context.Context, func() context.Context) {
	ctx, endOfConcurrency := WithConcurrencyEffectHandler(ctx, config.BufferSize)
	sink := make(chan TimeBoundedStatePayload, 2*config.NumWorkers)
	stateHandler := &stateHandler{
		stateMap: &sync.Map{},
		sink:     sink,
	}
	for k, v := range initMap {
		stateHandler.stateMap.Store(k, v)
	}
	return WithResumablePartitionableEffectHandler(
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
	resultCh := PerformResumableEffect[StatePayload, any](ctx, effectmodel.EffectState, payload)
	select {
	case res := <-resultCh:
		val = res.Value
		err = res.Err
	case <-ctx.Done():
	}
	return
}

// ErrKeyNotFound is an error indicating that the key was not found in any state handlers.
var ErrKeyNotFound = fmt.Errorf("key not found")

// delegateStateEffect is an internal helper for performing the state effect directly.
func delegateStateEffect(upperCtx context.Context, payload StatePayload) (res any, err error) {
	defer func() {
		if r := recover(); r != nil {
			if r, ok := r.(error); ok && errors.Is(r, ErrNoEffectHandler) {
				// Handle panic and return a nil value with an error
				// indicating that the effect handler is not available to delegate.
				res = nil
				err = ErrKeyNotFound
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
	stateMap *sync.Map
	sink     chan TimeBoundedStatePayload
}

// getStateEffect retrieves the value for the given key from the state map.
// If the key is not found, it delegates the effect to the upstream handler.
// It returns the value, error (if any), and a boolean indicating if the effect was delegated.
func (sH stateHandler) getStateEffect(ctx context.Context, payload GetStatePayload) (res any, err error, delegated bool) {
	v, ok := sH.stateMap.Load(payload.Key)
	if !ok {
		res, err = delegateStateEffect(ctx, payload)
		delegated = true
	} else {
		res = v
		err = nil
		delegated = false
	}
	return
}

var ErrOperationFailedWithMaxAttempts = fmt.Errorf("operation failed after max attempts")

// handle routes the given payload to the appropriate state operation logic.
func (sH stateHandler) handle(ctx context.Context, payload StatePayload) (any, error) {
	maxAttempts := 10
	switch payload := payload.(type) {

	case SetStatePayload:
		var res any
		var err error
		var delegated bool
		var payloadWithTimeSpan TimeBoundedStatePayload

		numAttempts := 0
		for {
			res, err, delegated = sH.getStateEffect(ctx, GetStatePayload{Key: payload.Key})
			if unknowsErr := err != nil && !errors.Is(err, ErrKeyNotFound); unknowsErr {
				return nil, err
			}
			if swapped := sH.stateMap.CompareAndSwap(payload.Key, res, payload.Value); swapped {
				payloadWithTimeSpan = statePayloadWithNow(payload)
				break
			}
			if numAttempts++; numAttempts >= maxAttempts {
				return nil, fmt.Errorf("%w: %d attempts", ErrOperationFailedWithMaxAttempts, numAttempts)
			}
		}

		newForAll := errors.Is(err, ErrKeyNotFound)
		newForLocal := delegated
		localDiff := !newForLocal && !payload.Value.Equals(res)
		if newForAll || newForLocal || localDiff {
			ConcurrencyEffect(ctx, func(ctx context.Context) {
				select {
				case <-ctx.Done():
				case sH.sink <- payloadWithTimeSpan:
				}
			})
		}

		return nil, nil

	case DeleteStatePayload:
		var res any
		var err error
		var delegated bool
		var payloadWithTimeSpan TimeBoundedStatePayload

		numAttempts := 0
		for {
			res, err, delegated = sH.getStateEffect(ctx, GetStatePayload{Key: payload.Key})
			if errors.Is(err, ErrKeyNotFound) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			if delegated {
				return nil, nil
			}
			if deleted := sH.stateMap.CompareAndDelete(payload.Key, res); deleted {
				payloadWithTimeSpan = statePayloadWithNow(payload)
				break
			}
			if numAttempts++; numAttempts >= maxAttempts {
				return nil, fmt.Errorf("%w: %d attempts", ErrOperationFailedWithMaxAttempts, numAttempts)
			}
		}

		ConcurrencyEffect(ctx, func(ctx context.Context) {
			select {
			case <-ctx.Done():
			case sH.sink <- payloadWithTimeSpan:
			}

		})

		return nil, nil

	case GetStatePayload:
		res, err, _ := sH.getStateEffect(ctx, payload)
		return res, err

	case SourceStatePayload:
		return sH.sink, nil

	default:
		// This should never happen because we are using a sealed interface to prevent adding new types.
		// But we still need to handle it to satisfy the compiler.
		// This is a safety net.
		// If this happens, it means that we have added a new type to the sealed interface
		// without updating the switch statement.
		// So we need to panic to avoid silent failures.
		// This is a bug in the code.
		// We need to fix it.
		panic(fmt.Errorf("invalid state operation type: %T", payload))
	}
}

// TimeBoundedStatePayload is a wrapper for StatePayload with a time span.
type TimeBoundedStatePayload struct {
	StatePayload
	TimeSpan
}

func statePayloadWithNow(payload StatePayload) TimeBoundedStatePayload {
	return TimeBoundedStatePayload{StatePayload: payload, TimeSpan: Now()}
}
