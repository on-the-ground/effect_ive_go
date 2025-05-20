package effects

import (
	"context"
	"log"

	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	"github.com/on-the-ground/effect_ive_go/effects/internal/helper"
	sharedHelper "github.com/on-the-ground/effect_ive_go/shared/helper"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

// WithFireAndForgetEffectHandler registers a fire-and-forget effect handler for a given effect enum.
//
// Suitable for one-shot effects like logging, telemetry, or background publishing.
// This handler executes without returning a result.
func WithFireAndForgetEffectHandler[P any](
	ctx context.Context,
	bufferSize int,
	enum effectmodel.EffectEnum,
	handleFn func(context.Context, P),
	teardown ...func(),
) (context.Context, func() context.Context) {
	td := normalizeTeardown(teardown)
	handler := handlers.NewFireAndForgetHandler(ctx, bufferSize, handleFn, td)
	ctxWith := context.WithValue(ctx, enum, handler)
	log.Printf("created fire/forget effect handler: effectId: %v, enum: %v", handler.EffectId, enum)

	return ctxWith, func() context.Context {
		handler.Close()
		log.Printf("closed fire/forget effect handler: effectId: %v, enum: %v", handler.EffectId, enum)
		return ctx
	}
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
	td := normalizeTeardown(teardown)
	handler := handlers.NewPartitionableFireAndForgetHandler(ctx, config, handleFn, td)
	ctxWith := context.WithValue(ctx, enum, handler)
	log.Printf("created fire/forget effect handler: effectId: %v, enum: %v", handler.EffectId, enum)

	return ctxWith, func() context.Context {
		handler.Close()
		log.Printf("closed fire/forget effect handler: effectId: %v, enum: %v", handler.EffectId, enum)
		return ctx
	}
}

// FireAndForgetEffect triggers a fire-and-forget effect for the given enum and payload.
//
// The handler will process the payload asynchronously.
// Panics if no handler is registered for the given enum.
func FireAndForgetEffect[P any](
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

func FireAndForgetEffectHandlerId[P any](
	ctx context.Context,
	enum effectmodel.EffectEnum,
) string {
	handler := sharedHelper.MustGetTypedValue[handlers.FireAndForgetHandler[P]](
		func() (any, error) {
			return helper.GetHandler(ctx, enum)
		},
	)
	return handler.EffectId
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
