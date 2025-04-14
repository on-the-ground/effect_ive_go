package effects

import (
	"context"

	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

// WithResumablePartitionableEffectHandler registers a resumable effect handler for a given effect enum.
//
// This handler supports hash-based partitioning via PartitionKey(), and is suitable for effects
// like state updates or messaging systems where per-key ordering matters.
//
// Usage:
//
//	ctx, cancel := WithResumablePartitionableEffectHandler(ctx, config, MyEffectEnum, handleFn)
//	defer cancel()
func WithResumablePartitionableEffectHandler[T effectmodel.Partitionable, R any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	enum effectmodel.EffectEnum,
	handleFn func(context.Context, handlers.ResumableEffectMessage[T, R]),
	teardown ...func(),
) (context.Context, func()) {
	td := normalizeTeardown(teardown)
	handler := handlers.NewPartitionableResumableHandler(ctx, config, handleFn, td)
	ctxWith := context.WithValue(ctx, enum, handler)
	LogEffect(ctxWith, LogDebug, "created resumable effect handler", map[string]interface{}{
		"effectId": handler.EffectId,
		"enum":     enum,
	})
	return ctxWith, func() {
		handler.Close()
		LogEffect(ctxWith, LogDebug, "closed resumable effect handler", map[string]interface{}{
			"effectId": handler.EffectId,
			"enum":     enum,
		})
	}
}

// PerformResumableEffect sends a payload to the resumable effect handler and waits for the result.
//
// It returns the value sent through resumeCh by the handler logic.
// Panics if no handler is registered for the given effect enum.
func PerformResumableEffect[T effectmodel.Partitionable, R any](
	ctx context.Context,
	enum effectmodel.EffectEnum,
	payload T,
) R {
	handler := mustGetTypedValue[handlers.ResumableHandler[T, R]](
		func() (any, error) {
			return hasHandler(ctx, enum)
		},
	)
	return handler.PerformEffect(ctx, payload)
}

// WithFireAndForgetEffectHandler registers a fire-and-forget effect handler for a given effect enum.
//
// Suitable for one-shot effects like logging, telemetry, or background publishing.
// This handler executes without returning a result.
func WithFireAndForgetEffectHandler[T any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	enum effectmodel.EffectEnum,
	handleFn func(context.Context, T),
	teardown ...func(),
) (context.Context, func()) {
	td := normalizeTeardown(teardown)
	handler := handlers.NewFireAndForgetHandler(ctx, config, handleFn, td)
	ctxWith := context.WithValue(ctx, enum, handler)
	LogEffect(ctxWith, LogDebug, "created fire/forget effect handler", map[string]interface{}{
		"effectId": handler.EffectId,
		"enum":     enum,
	})
	return ctxWith, func() {
		handler.Close()
		LogEffect(ctxWith, LogDebug, "closed fire/forget effect handler", map[string]interface{}{
			"effectId": handler.EffectId,
			"enum":     enum,
		})
	}
}

// FireAndForgetEffect triggers a fire-and-forget effect for the given enum and payload.
//
// The handler will process the payload asynchronously.
// Panics if no handler is registered for the given enum.
func FireAndForgetEffect[T any](
	ctx context.Context,
	enum effectmodel.EffectEnum,
	payload T,
) {
	handler := mustGetTypedValue[handlers.FireAndForgetHandler[T]](
		func() (any, error) {
			return hasHandler(ctx, enum)
		},
	)
	handler.FireAndForgetEffect(ctx, payload)
}

// WithFireAndForgetPartitionableEffectHandler registers a partitioned fire-and-forget handler.
//
// Hash-based dispatching ensures that effects with the same PartitionKey() are handled
// by the same goroutine. Useful for ensuring ordering by key.
func WithFireAndForgetPartitionableEffectHandler[T effectmodel.Partitionable](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	enum effectmodel.EffectEnum,
	handleFn func(context.Context, T),
	teardown ...func(),
) (context.Context, func()) {
	td := normalizeTeardown(teardown)
	handler := handlers.NewPartitionableFireAndForgetHandler(ctx, config, handleFn, td)
	ctxWith := context.WithValue(ctx, enum, handler)
	LogEffect(ctxWith, LogDebug, "created fire/forget effect handler", map[string]interface{}{
		"effectId": handler.EffectId,
		"enum":     enum,
	})
	return ctxWith, func() {
		handler.Close()
		LogEffect(ctxWith, LogDebug, "closed fire/forget effect handler", map[string]interface{}{
			"effectId": handler.EffectId,
			"enum":     enum,
		})
	}
}

// normalizeTeardown flattens optional teardown functions into a single callable.
//
// Accepts either 0 or 1 teardown functions. Panics if more than one is passed.
func normalizeTeardown(teardown []func()) func() {
	switch len(teardown) {
	case 1:
		return teardown[0]
	case 0:
		return func() {}
	default:
		panic("normalizeTeardown: only one teardown function allowed")
	}
}
