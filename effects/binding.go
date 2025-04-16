package effects

import (
	"context"
	"fmt"

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

// WithBindingEffectHandler registers a resumable, partitionable effect handler for bindings.
//
// - Reads buffer/worker config via BindingEffect itself (bootstrapped).
// - If no config is found, defaults are used (bufferSize = 1, numWorkers = 1).
// - Accepts a key-value map used for lookups.
// - Allows fallback to upper scopes if a key is not found locally.
func WithBindingEffectHandler(
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	bindingMap map[string]any,
) (context.Context, func() context.Context) {
	bindingHandler := &bindingHandler{
		bindingMap: normalizeBindingMap(bindingMap),
	}
	return WithResumablePartitionableEffectHandler(
		ctx,
		config,
		effectmodel.EffectBinding,
		bindingHandler.handle,
	)
}

// BindingEffect performs a key-based lookup using the Binding effect handler.
//
// Returns either the value found or an error if the key is not found and no upper scope provides it.
func BindingEffect(ctx context.Context, payload BindingPayload) (val any, err error) {
	resultCh := PerformResumableEffect[BindingPayload, any](ctx, effectmodel.EffectBinding, payload)
	select {
	case res := <-resultCh:
		val = res.Value
		err = res.Err
	case <-ctx.Done():
	}
	return
}

// normalizeTeardown is an internal helper for normalizing binding map.
func normalizeBindingMap(bm map[string]any) map[string]any {
	if bm == nil {
		bm = make(map[string]any)
	}
	return bm
}

// delegateBindingEffect is an internal helper for performing the binding effect directly.
func delegateBindingEffect(upperCtx context.Context, payload BindingPayload) (res any, err error) {
	defer func() {
		if r := recover(); r != nil {
			// Handle panic and return a nil value with an error
			// indicating that the effect handler is not available to delegate.
			res = nil
			err = fmt.Errorf("key not found: %v", r)
		}
	}()

	// Delegate the effect to the upper handler
	return BindingEffect(upperCtx, payload)
}

// bindingHandler
type bindingHandler struct {
	bindingMap map[string]any
}

// handle looks up the key in the local bindingMap.
// - If found: returns the value.
// - If not found: attempts to delegate the effect to an upper handler (if available).
// - Otherwise: returns a key-not-found error.
func (bh bindingHandler) handle(ctx context.Context, payload BindingPayload) (any, error) {
	v, ok := bh.bindingMap[payload.Key]
	if !ok {
		return delegateBindingEffect(ctx, payload)
	}
	return v, nil
}
