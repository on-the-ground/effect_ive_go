package effects

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	sharedHelper "github.com/on-the-ground/effect_ive_go/shared/helper"
)

// WithResumableEffectHandler registers a resumable effect handler for a given effectKey.
//
// This handler is suitable for effects that don't require partitioning.
// It can be used for one-shot effects or those that don't require ordering by key.
func WithResumableEffectHandler[P any](
	pctx context.Context,
	bufferSize int,
	effectKey EffectKey,
	handleFn func(context.Context, P) (any, error),
	teardown ...func(),
) (context.Context, func() context.Context) {
	td := normalizeTeardown(teardown)
	ctxForHandler, cancelHandler := context.WithCancel(pctx)
	handler := handlers.NewResumableHandler(
		ctxForHandler,
		uuid.New().String(),
		bufferSize,
		handleFn,
	)

	ctx := context.WithValue(pctx, effectKey, handler)
	log.Printf("created resumable effect handler: effectId: %v, effectKey: %v", handler.EffectId, effectKey)

	return ctx, func() context.Context {
		td()
		cancelHandler()
		log.Printf("closed resumable effect handler: effectId: %v, effectKey:%v", handler.EffectId, effectKey)
		return pctx
	}
}

// WithResumablePartitionableEffectHandler registers a resumable effect handler for a given effectKey.
//
// This handler supports hash-based partitioning via PartitionKey(), and is suitable for effects
// like state updates or messaging systems where per-key ordering matters.
//
// Usage:
//
//	ctx, cancel := WithResumablePartitionableEffectHandler(ctx, config, MyEffectKey, handleFn)
//	defer cancel()
func WithResumablePartitionableEffectHandler[P effectmodel.Partitionable](
	pctx context.Context,
	bufferSize, numWorkers int,
	effectKey EffectKey,
	handleFn func(context.Context, P) (any, error),
	teardown ...func(),
) (context.Context, func() context.Context) {
	td := normalizeTeardown(teardown)
	ctxForHandler, cancelHandler := context.WithCancel(pctx)
	handler := handlers.NewResumablePartitionableHandler(
		ctxForHandler,
		uuid.New().String(),
		numWorkers,
		bufferSize,
		handleFn,
	)
	ctx := context.WithValue(pctx, effectKey, handler)
	log.Printf("created resumable effect handler: effectId: %v, effectKey: %v", handler.EffectId, effectKey)

	return ctx, func() context.Context {
		td()
		cancelHandler()
		log.Printf("closed resumable effect handler: effectId: %v, effectKey:%v", handler.EffectId, effectKey)
		return pctx
	}
}

// PerformResumableEffect sends a payload to the resumable effect handler and waits for the result.
//
// It returns the value sent through resumeCh by the handler logic.
// Panics if no handler is registered for the given effectKey.
func PerformResumableEffect[P any](
	ctx context.Context,
	effectKey EffectKey,
	payload P,
) <-chan handlers.ResumableResult {
	handler := sharedHelper.MustGetTypedValue[handlers.ResumableHandler[P]](
		func() (any, error) {
			return GetHandler(ctx, effectKey)
		},
	)
	return handler.PerformEffect(ctx, payload)
}

func ResumableEffectHandlerId[P any](
	ctx context.Context,
	effectKey EffectKey,
) string {
	handler := sharedHelper.MustGetTypedValue[handlers.ResumableHandler[P]](
		func() (any, error) {
			return GetHandler(ctx, effectKey)
		},
	)
	return handler.EffectId
}
