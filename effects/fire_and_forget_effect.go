package effects

import (
	"context"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	sharedHelper "github.com/on-the-ground/effect_ive_go/shared/helper"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

type EffectKey string

// WithFireAndForgetEffectHandler registers a fire-and-forget effect handler for a given effectKey.
//
// Suitable for one-shot effects like logging, telemetry, or background publishing.
// This handler executes without returning a result.
func WithFireAndForgetEffectHandler[P any](
	pctx context.Context,
	bufferSize int,
	effectKey EffectKey,
	handleFn func(context.Context, P),
	teardown ...func(),
) (context.Context, func() context.Context) {
	td := normalizeTeardown(teardown)
	ctxForHandler, cancelHandler := context.WithCancel(pctx)
	handler := handlers.NewFireAndForgetHandler(
		ctxForHandler,
		uuid.New().String(),
		bufferSize,
		handleFn,
	)

	ctx := context.WithValue(pctx, effectKey, handler)
	log.Printf("created fire/forget effect handler: effectId: %v, effectKey: %v", handler.EffectId, effectKey)

	var once sync.Once

	return ctx, func() context.Context {
		ran := false
		once.Do(func() {
			ran = true
			// Stop intake first, then teardown.
			cancelHandler()
			td()
			log.Printf("closed fireNforget effect handler: effectId: %v, effectKey: %v", handler.EffectId, effectKey)
		})
		if !ran {
			// Someone already closed it. Make the log match the handler type.
			log.Printf("fireNforget effect handler already closed: effectId: %v, effectKey: %v", handler.EffectId, effectKey)
		}
		return pctx
	}
}

// WithFireAndForgetPartitionableEffectHandler registers a partitioned fire-and-forget handler.
//
// Hash-based dispatching ensures that effects with the same PartitionKey() are handled
// by the same goroutine. Useful for ensuring ordering by key.
func WithFireAndForgetPartitionableEffectHandler[P effectmodel.Partitionable](
	pctx context.Context,
	bufferSize, numWorkers int,
	effectKey EffectKey,
	handleFn func(context.Context, P),
	teardown ...func(),
) (context.Context, func() context.Context) {
	td := normalizeTeardown(teardown)
	ctxForHandler, cancelHandler := context.WithCancel(pctx)
	handler := handlers.NewFireAndForgetPartitionableHandler(
		ctxForHandler,
		uuid.New().String(),
		numWorkers,
		bufferSize,
		handleFn,
	)

	ctx := context.WithValue(pctx, effectKey, handler)
	log.Printf("created fire/forget effect handler: effectId: %v, effectKey: %v", handler.EffectId, effectKey)

	var once sync.Once

	return ctx, func() context.Context {
		ran := false
		once.Do(func() {
			ran = true
			// Stop intake first, then teardown.
			cancelHandler()
			td()
			log.Printf("closed fireNforget effect handler: effectId: %v, effectKey: %v", handler.EffectId, effectKey)
		})
		if !ran {
			// Someone already closed it. Make the log match the handler type.
			log.Printf("fireNforget effect handler already closed: effectId: %v, effectKey: %v", handler.EffectId, effectKey)
		}
		return pctx
	}
}

// FireAndForgetEffect triggers a fire-and-forget effect for the given effectKey and payload.
//
// The handler will process the payload asynchronously.
// Panics if no handler is registered for the given effectKey.
func FireAndForgetEffect[P any](
	ctx context.Context,
	effectKey EffectKey,
	payload P,
) {
	handler := sharedHelper.MustGetTypedValue[handlers.FireAndForgetHandler[P]](
		func() (any, error) {
			return GetHandler(ctx, effectKey)
		},
	)
	handler.FireAndForgetEffect(ctx, payload)
}

func FireAndForgetEffectHandlerId[P any](
	ctx context.Context,
	effectKey EffectKey,
) string {
	handler := sharedHelper.MustGetTypedValue[handlers.FireAndForgetHandler[P]](
		func() (any, error) {
			return GetHandler(ctx, effectKey)
		},
	)
	return handler.EffectId
}
