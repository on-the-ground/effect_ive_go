package handlers

import (
	"context"
	"log"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

func NewFireAndForgetHandler[T any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, T),
	teardown func(),
) FireAndForgetHandler[T] {
	ctx, cancelFn := context.WithCancel(ctx)
	queue := NewSingleQueue(ctx, config.BufferSize, handleFn)
	return FireAndForgetHandler[T]{
		effectScope: newEffectScope(ctx, queue, func() {
			cancelFn()
			teardown()
		}),
	}
}

func NewPartitionableFireAndForgetHandler[T effectmodel.Partitionable](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, T),
	teardown func(),
) FireAndForgetHandler[T] {
	ctx, cancelFn := context.WithCancel(ctx)
	queue := NewPartitionedQueue(ctx, config.NumWorkers, config.BufferSize, handleFn)
	return FireAndForgetHandler[T]{
		effectScope: newEffectScope(ctx, queue, func() {
			cancelFn()
			teardown()
		}),
	}
}

type FireAndForgetHandler[T any] struct {
	*effectScope[T]
}

func (ffh FireAndForgetHandler[T]) FireAndForgetEffect(ctx context.Context, payload T) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf(
				"panic while sending to closed channel for effect: %+v",
				map[string]interface{}{
					"effectId": ffh.EffectId,
					"payload":  payload,
				},
			)
		}
	}()

	select {
	case <-ctx.Done():
	case ffh.queue.GetChannelOf(payload) <- payload:
	}
}
