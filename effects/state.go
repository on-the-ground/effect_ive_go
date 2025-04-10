package effects

import (
	"context"
	"fmt"
	"sync"

	"github.com/on-the-ground/effect_ive_go/effects/configkeys"
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
func WithStateEffectHandler(ctx context.Context, initMap map[string]any) (context.Context, func()) {
	bufferSize, err := GetFromBindingEffect[int](ctx, configkeys.ConfigEffectStateHandlerBufferSize)
	if err != nil {
		bufferSize = 1
	}
	numWorkers, err := GetFromBindingEffect[int](ctx, configkeys.ConfigEffectStateHandlerNumWorkers)
	if err != nil {
		numWorkers = 1
	}

	stateHandler := &stateHandler{
		stateMap: &sync.Map{},
	}
	for k, v := range initMap {
		stateHandler.stateMap.Store(k, v)
	}
	return WithResumablePartitionableEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(bufferSize, numWorkers),
		effectmodel.EffectState,
		func(ctx context.Context, msg handlers.ResumableEffectMessage[statePayload, StateResult]) {
			msg.ResumeCh <- stateHandler.handleFn(ctx, msg.Payload)
		},
	)
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
		_, err := hasHandler(ctx, effectmodel.EffectState)
		if err != nil {
			return stateEffect(ctx, payload)
		}
		return StateResult{value: nil, err: fmt.Errorf("key not found: %s", payload.Key)}
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
