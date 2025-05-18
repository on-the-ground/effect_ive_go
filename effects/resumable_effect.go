package effects

import (
	"context"

	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	"github.com/on-the-ground/effect_ive_go/effects/internal/helper"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	sharedHelper "github.com/on-the-ground/effect_ive_go/shared/helper"
	"go.uber.org/zap"
)

// WithResumableEffectHandler registers a resumable effect handler for a given effect enum.
//
// This handler is suitable for effects that don't require partitioning.
// It can be used for one-shot effects or those that don't require ordering by key.
func WithResumableEffectHandler[P any, R any](
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

// PerformResumableEffect sends a payload to the resumable effect handler and waits for the result.
//
// It returns the value sent through resumeCh by the handler logic.
// Panics if no handler is registered for the given effect enum.
func PerformResumableEffect[P any, R any](
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

func ResumableEffectHandlerId[P any, R any](
	ctx context.Context,
	enum effectmodel.EffectEnum,
) string {
	handler := sharedHelper.MustGetTypedValue[handlers.ResumableHandler[P, R]](
		func() (any, error) {
			return helper.GetHandler(ctx, enum)
		},
	)
	return handler.EffectId
}
