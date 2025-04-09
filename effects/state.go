package effects

import (
	"context"
	"fmt"
	"sync"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

var _ statePayload = GetStatePayload{}

type GetStatePayload struct {
	Key string
}

func (p GetStatePayload) PartitionKey() string {
	return p.Key
}

func (p GetStatePayload) sealedInterface() {}

var _ statePayload = DeleteStatePayload{}

type DeleteStatePayload struct {
	Key string
}

func (p DeleteStatePayload) PartitionKey() string {
	return p.Key
}

func (p DeleteStatePayload) sealedInterface() {}

var _ statePayload = SetStatePayload{}

type SetStatePayload struct {
	Key   string
	Value any
}

func (p SetStatePayload) PartitionKey() string {
	return p.Key
}

func (p SetStatePayload) sealedInterface() {}

type statePayload interface {
	PartitionKey() string
	sealedInterface()
}

type StateResult struct {
	value any
	err   error
}

func WithStateEffectHandler(ctx context.Context, initMap map[string]any) (context.Context, func()) {
	bufferSize := MustGetFromBindingEffect[int](ctx, "config.effect.state.handler.buffer_size")
	numWorkers := MustGetFromBindingEffect[int](ctx, "config.effect.state.handler.num_workers")

	stateHandler := &stateHandler{
		stateMap: &sync.Map{},
	}
	for k, v := range initMap {
		stateHandler.stateMap.Store(k, v)
	}
	return WithResumableEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(bufferSize, numWorkers),
		effectmodel.EffectState,
		func(ctx context.Context, payload statePayload) StateResult {
			return stateHandler.handleFn(ctx, payload)
		},
	)
}

func stateEffect(ctx context.Context, payload statePayload) StateResult {
	return PerformResumableEffect[statePayload, StateResult](ctx, effectmodel.EffectState, payload)
}

func StateEffect(ctx context.Context, payload statePayload) (any, error) {
	res := stateEffect(ctx, payload)
	return res.value, res.err
}

// stateHandler
type stateHandler struct {
	stateMap *sync.Map
}

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
