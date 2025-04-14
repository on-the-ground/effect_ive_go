package effects

import (
	"context"
	"fmt"
	"sync"

	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

var _ statePayload = GetStatePayload{}

// GetStatePayload is the payload type for retrieving a value from the state.
type GetStatePayload struct {
	Key string
}

// PartitionKey returns the partition key for routing this payload.
func (p GetStatePayload) PartitionKey() string { return p.Key }

// sealedInterfaceStatePayload prevents external packages from implementing statePayload.
func (p GetStatePayload) sealedInterfaceStatePayload() {}

var _ statePayload = DeleteStatePayload{}

// DeleteStatePayload is the payload type for deleting a key from the state.
type DeleteStatePayload struct {
	Key string
}

func (p DeleteStatePayload) PartitionKey() string         { return p.Key }
func (p DeleteStatePayload) sealedInterfaceStatePayload() {}

var _ statePayload = SetStatePayload{}

// SetStatePayload is the payload type for inserting or updating a key-value pair.
type SetStatePayload struct {
	Key   string
	Value any
}

func (p SetStatePayload) PartitionKey() string         { return p.Key }
func (p SetStatePayload) sealedInterfaceStatePayload() {}

// statePayload is a sealed interface for state operations.
// Only predefined payload types (Set, Get, Delete) can implement this interface.
type statePayload interface {
	PartitionKey() string
	sealedInterfaceStatePayload()
}

// StateResult represents the result of a state operation.
type StateResult struct {
	value any
	err   error
}

// WithStateEffectHandler registers a resumable, partitionable effect handler for managing key-value state.
// It stores the internal state in a memory-safe sync.Map and supports sharded processing.
func WithStateEffectHandler(
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	initMap map[string]any,
) (context.Context, func()) {
	stateHandler := &stateHandler{
		stateMap: &sync.Map{},
	}
	for k, v := range initMap {
		stateHandler.stateMap.Store(k, v)
	}
	return WithResumablePartitionableEffectHandler(
		ctx,
		config,
		effectmodel.EffectState,
		func(ctx context.Context, msg handlers.ResumableEffectMessage[statePayload, StateResult]) {
			msg.ResumeCh <- stateHandler.handleFn(ctx, msg.Payload)
		},
	)
}

// delegateStateEffect is an internal helper for performing the state effect directly.
func delegateStateEffect(ctx context.Context, payload statePayload) (res StateResult) {
	defer func() {
		if r := recover(); r != nil {
			// Handle panic and return a nil value with an error
			// indicating that the effect handler is not available to delegate.
			res = StateResult{
				value: nil,
				err:   fmt.Errorf("key not found: %v", r),
			}
		}
	}()
	return stateEffect(ctx, payload)
}

func stateEffect(ctx context.Context, payload statePayload) StateResult {
	return PerformResumableEffect[statePayload, StateResult](ctx, effectmodel.EffectState, payload)
}

// StateEffect performs a state operation (get, set, delete) using the EffectState handler.
func StateEffect(ctx context.Context, payload statePayload) (any, error) {
	res := stateEffect(ctx, payload)
	return res.value, res.err
}

// stateHandler defines the in-memory state store logic.
// It supports safe concurrent access and fallback to upstream handler if key is missing.
type stateHandler struct {
	stateMap *sync.Map
}

// handleFn routes the given payload to the appropriate state operation logic.
func (sH stateHandler) handleFn(ctx context.Context, payload statePayload) StateResult {
	switch payload := payload.(type) {
	case SetStatePayload:
		sH.stateMap.Store(payload.Key, payload.Value)
		return StateResult{value: nil, err: nil}
	case DeleteStatePayload:
		sH.stateMap.Delete(payload.Key)
		return StateResult{value: nil, err: nil}
	case GetStatePayload:
		v, ok := sH.stateMap.Load(payload.Key)
		if ok {
			return StateResult{value: v, err: nil}
		}
		return delegateStateEffect(ctx, payload)
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
