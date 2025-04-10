package effects

import (
	"context"
	"fmt"

	"github.com/on-the-ground/effect_ive_go/effects/configkeys"
	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

// BindingPayload defines a key-based lookup payload.
// Used as input to the Binding effect.
type BindingPayload struct {
	Key string
}

// BindingPayload defines a key-based lookup payload.
// Used as input to the Binding effect.
func (bp BindingPayload) PartitionKey() string {
	return bp.Key
}

// BindingResult is the result of a binding lookup.
// It contains either a value or an error if the key was not found.
type BindingResult struct {
	value any
	err   error
}

// WithBindingEffectHandler registers a resumable, partitionable effect handler for bindings.
//
// - Reads buffer/worker config via BindingEffect itself (bootstrapped).
// - If no config is found, defaults are used (bufferSize = 1, numWorkers = 1).
// - Accepts a key-value map used for lookups.
// - Allows fallback to upper scopes if a key is not found locally.
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
	return WithResumablePartitionableEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(bufferSize, numWorkers),
		effectmodel.EffectBinding,
		func(ctx context.Context, msg handlers.ResumableEffectMessage[BindingPayload, BindingResult]) {
			msg.ResumeCh <- bindingHandler.handleFn(ctx, msg.Payload)
		},
	)
}

// bindingEffect is an internal helper for performing the binding effect directly.
func bindingEffect(ctx context.Context, payload BindingPayload) BindingResult {
	return PerformResumableEffect[BindingPayload, BindingResult](ctx, effectmodel.EffectBinding, payload)
}

// BindingEffect performs a key-based lookup using the Binding effect handler.
//
// Returns either the value found or an error if the key is not found and no upper scope provides it.
func BindingEffect(ctx context.Context, payload BindingPayload) (any, error) {
	res := bindingEffect(ctx, payload)
	return res.value, res.err
}

// bindingHandler
type bindingHandler struct {
	bindingMap map[string]any
}

// handleFn looks up the key in the local bindingMap.
// - If found: returns the value.
// - If not found: attempts to delegate the effect to an upper handler (if available).
// - Otherwise: returns a key-not-found error.
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
