package handlers

import (
	"context"
	"log"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

func NewFireAndForgetHandler[T any](
	ctx context.Context,
	bufferSize int,
	handleFn func(context.Context, T),
	teardown func(),
) FireAndForgetHandler[T] {
	ctx, cancelFn := context.WithCancel(ctx)
	return FireAndForgetHandler[T]{
		effectScope: newEffectScope(
			NewSingleQueue(ctx, bufferSize, handleFn),
			func() {
				teardown()
				cancelFn()
			},
		),
	}
}

func NewPartitionableFireAndForgetHandler[T effectmodel.Partitionable](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, T),
	teardown func(),
) FireAndForgetHandler[T] {
	ctx, cancelFn := context.WithCancel(ctx)
	return FireAndForgetHandler[T]{
		effectScope: newEffectScope(
			NewPartitionedQueue(ctx, config.NumWorkers, config.BufferSize, handleFn),
			func() {
				teardown()
				cancelFn()
			},
		),
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
	case ffh.dispatcher.GetChannelOf(payload) <- payload:
	}
}
