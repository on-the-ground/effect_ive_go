package binding

import (
	"context"
	"fmt"

	"github.com/on-the-ground/effect_ive_go/effects"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

// Payload defines a key-based lookup payload.
// Used as input to the Binding effect.
type Payload string

// BindingPayload defines a key-based lookup payload.
// Used as input to the Binding effect.
func (bp Payload) PartitionKey() string {
	return string(bp)
}

// WithEffectHandler registers a resumable, partitionable effect handler for bindings.
//
//   - Reads buffer/worker config via BindingEffect itself (bootstrapped).
//   - If no config is found, defaults are used (bufferSize = 1, numWorkers = 1).
//   - Accepts a key-value map used for lookups.
//   - Allows fallback to upper scopes if a key is not found locally.
//   - Returns a context with the effect handler registered.
//   - Returns a teardown function to close the handler.
//   - The teardown function should be called when the effect handler is no longer needed.
//   - If the teardown function is called early, the effect handler will be closed,
//     you should use the context returned by the teardown function.
func WithEffectHandler(
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	bindingMap map[string]any,
) (context.Context, func() context.Context) {
	bindingHandler := &bindingHandler{
		bindingMap: normalizeBindingMap(bindingMap),
	}
	return effects.WithResumablePartitionableEffectHandler[Payload, any](
		ctx,
		config,
		effectmodel.EffectBinding,
		bindingHandler.handle,
	)
}

// BindingEffect performs a key-based lookup using the Binding effect handler.
//
// Returns either the value found or an error if the key is not found and no upper scope provides it.
func Effect(ctx context.Context, key string) (val any, err error) {
	resultCh := effects.PerformResumableEffect[Payload, any](ctx, effectmodel.EffectBinding, Payload(key))
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

// normalizeTeardown is an internal helper for normalizing binding map.
func normalizeBindingMap(bm map[string]any) map[string]any {
	if bm == nil {
		bm = make(map[string]any)
	}
	return bm
}

// delegateBindingEffect is an internal helper for performing the binding effect directly.
func delegateBindingEffect(upperCtx context.Context, key string) (res any, err error) {
	defer func() {
		if r := recover(); r != nil {
			// Handle panic and return a nil value with an error
			// indicating that the effect handler is not available to delegate.
			res = nil
			err = fmt.Errorf("key not found: %v", r)
		}
	}()

	// Delegate the effect to the upper handler
	return Effect(upperCtx, key)
}

// bindingHandler
type bindingHandler struct {
	bindingMap map[string]any
}

// handle looks up the key in the local bindingMap.
// - If found: returns the value.
// - If not found: attempts to delegate the effect to an upper handler (if available).
// - Otherwise: returns a key-not-found error.
func (bh bindingHandler) handle(ctx context.Context, payload Payload) (any, error) {
	key := string(payload)
	v, ok := bh.bindingMap[key]
	if !ok {
		return delegateBindingEffect(ctx, key)
	}
	return v, nil
}
