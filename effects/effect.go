package effects

import (
	"context"

	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	"github.com/on-the-ground/effect_ive_go/effects/internal/helper"
	sharedHelper "github.com/on-the-ground/effect_ive_go/shared/helper"
	"go.uber.org/zap"

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
func WithResumablePartitionableEffectHandler[P effectmodel.Partitionable, R any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	enum effectmodel.EffectEnum,
	handleFn func(context.Context, P) (R, error),
	teardown ...func(),
) (context.Context, func() context.Context) {
	logger, _ := zap.NewProduction()
	td := normalizeTeardown(teardown)
	handler := handlers.NewPartitionableResumableHandler(ctx, config, handleFn, td)
	ctxWith := context.WithValue(ctx, enum, handler)
	logger.Sugar().Debugf("created resumable effect handler: effectId: %v, enum: %v", handler.EffectId, enum)

	return ctxWith, func() context.Context {
		handler.Close()
		logger.Sugar().Debugf("closed resumable effect handler: effectId: %v, enum:%v", handler.EffectId, enum)
		return ctx
	}
}

// WithResumableEffectHandler registers a resumable effect handler for a given effect enum.
//
// This handler is suitable for effects that don't require partitioning.
// It can be used for one-shot effects or those that don't require ordering by key.
func WithResumableEffectHandler[P effectmodel.Partitionable, R any](
	ctx context.Context,
	bufferSize int,
	enum effectmodel.EffectEnum,
	handleFn func(context.Context, P) (R, error),
	teardown ...func(),
) (context.Context, func() context.Context) {
	logger, _ := zap.NewProduction()
	td := normalizeTeardown(teardown)
	handler := handlers.NewResumableHandler(ctx, bufferSize, handleFn, td)
	ctxWith := context.WithValue(ctx, enum, handler)
	logger.Sugar().Debugf("created resumable effect handler: effectId: %v, enum: %v", handler.EffectId, enum)

	return ctxWith, func() context.Context {
		handler.Close()
		logger.Sugar().Debugf("closed resumable effect handler: effectId: %v, enum:%v", handler.EffectId, enum)
		return ctx
	}
}

// PerformResumableEffect sends a payload to the resumable effect handler and waits for the result.
//
// It returns the value sent through resumeCh by the handler logic.
// Panics if no handler is registered for the given effect enum.
func PerformResumableEffect[P effectmodel.Partitionable, R any](
	ctx context.Context,
	enum effectmodel.EffectEnum,
	payload P,
) <-chan handlers.ResumableResult[R] {
	handler := sharedHelper.MustGetTypedValue[handlers.ResumableHandler[P, R]](
		func() (any, error) {
			return helper.GetHandler(ctx, enum)
		},
	)
	return handler.PerformEffect(ctx, payload)
}

// WithFireAndForgetEffectHandler registers a fire-and-forget effect handler for a given effect enum.
//
// Suitable for one-shot effects like logging, telemetry, or background publishing.
// This handler executes without returning a result.
func WithFireAndForgetEffectHandler[P effectmodel.Partitionable](
	ctx context.Context,
	bufferSize int,
	enum effectmodel.EffectEnum,
	handleFn func(context.Context, P),
	teardown ...func(),
) (context.Context, func() context.Context) {
	logger, _ := zap.NewProduction()
	td := normalizeTeardown(teardown)
	handler := handlers.NewFireAndForgetHandler(ctx, bufferSize, handleFn, td)
	ctxWith := context.WithValue(ctx, enum, handler)
	logger.Sugar().Debugf("created fire/forget effect handler: effectId: %v, enum: %v", handler.EffectId, enum)

	return ctxWith, func() context.Context {
		handler.Close()
		logger.Sugar().Debugf("closed fire/forget effect handler: effectId: %v, enum: %v", handler.EffectId, enum)
		return ctx
	}
}

// FireAndForgetEffect triggers a fire-and-forget effect for the given enum and payload.
//
// The handler will process the payload asynchronously.
// Panics if no handler is registered for the given enum.
func FireAndForgetEffect[P effectmodel.Partitionable](
	ctx context.Context,
	enum effectmodel.EffectEnum,
	payload P,
) {
	handler := sharedHelper.MustGetTypedValue[handlers.FireAndForgetHandler[P]](
		func() (any, error) {
			return helper.GetHandler(ctx, enum)
		},
	)
	handler.FireAndForgetEffect(ctx, payload)
}

// WithFireAndForgetPartitionableEffectHandler registers a partitioned fire-and-forget handler.
//
// Hash-based dispatching ensures that effects with the same PartitionKey() are handled
// by the same goroutine. Useful for ensuring ordering by key.
func WithFireAndForgetPartitionableEffectHandler[P effectmodel.Partitionable](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	enum effectmodel.EffectEnum,
	handleFn func(context.Context, P),
	teardown ...func(),
) (context.Context, func() context.Context) {
	logger, _ := zap.NewProduction()
	td := normalizeTeardown(teardown)
	handler := handlers.NewPartitionableFireAndForgetHandler(ctx, config, handleFn, td)
	ctxWith := context.WithValue(ctx, enum, handler)
	logger.Sugar().Debugf("created fire/forget effect handler: effectId: %v, enum: %v", handler.EffectId, enum)

	return ctxWith, func() context.Context {
		handler.Close()
		logger.Sugar().Debugf("closed fire/forget effect handler: effectId: %v, enum: %v", handler.EffectId, enum)
		return ctx
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
		panic("normalizeTeardown: only one or zero teardown functions allowed")
	}
}
