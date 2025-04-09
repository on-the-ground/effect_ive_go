package effects

import (
	"context"
	"fmt"

	"github.com/on-the-ground/effect_ive_go/effects/configkeys"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

type BindingPayload struct {
	Key string
}

func (bp BindingPayload) PartitionKey() string {
	return bp.Key
}

type BindingResult struct {
	value any
	err   error
}

func WithBindingEffectHandler(ctx context.Context, bindingMap map[string]any) (context.Context, func()) {
	bufferSize, err := GetFromBindingEffect[int](ctx, configkeys.ConfigEffectBindingHandlerBufferSize)
	if err != nil {
		bufferSize = 1
	}

	numWorkers, err := GetFromBindingEffect[int](ctx, configkeys.ConfigEffectBindingHandlerNumWorkers)
	if err != nil {
		numWorkers = 1
	}
	if bindingMap == nil {
		bindingMap = make(map[string]any)
	}

	bindingHandler := &bindingHandler{
		bindingMap: bindingMap,
	}
	return WithResumableEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(bufferSize, numWorkers),
		effectmodel.EffectBinding,
		func(ctx context.Context, payload BindingPayload) BindingResult {
			return bindingHandler.handleFn(ctx, payload)
		},
	)
}

func bindingEffect(ctx context.Context, payload BindingPayload) BindingResult {
	return PerformResumableEffect[BindingPayload, BindingResult](ctx, effectmodel.EffectBinding, payload)
}

func BindingEffect(ctx context.Context, payload BindingPayload) (any, error) {
	res := bindingEffect(ctx, payload)
	return res.value, res.err
}

// bindingHandler
type bindingHandler struct {
	bindingMap map[string]any
}

func (bh bindingHandler) handleFn(ctx context.Context, payload BindingPayload) BindingResult {
	v, ok := bh.bindingMap[payload.Key]
	if ok {
		return BindingResult{value: v, err: nil}
	}
	_, err := hasHandler(ctx, effectmodel.EffectBinding)
	if err != nil {
		return bindingEffect(ctx, payload)
	}
	return BindingResult{value: nil, err: fmt.Errorf("key not found: %s", payload.Key)}
}
