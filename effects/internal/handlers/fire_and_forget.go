package handlers

import (
	"context"
	"log"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

type FireAndForgetHandler[P any] struct {
	EffectId   string
	dispatcher WorkerDispatcher[P]
}

func NewFireAndForgetHandler[P any](
	ctx context.Context,
	effectId string,
	bufferSize int,
	handleFn func(context.Context, P),
) FireAndForgetHandler[P] {
	return FireAndForgetHandler[P]{
		EffectId:   effectId,
		dispatcher: NewSingleQueue(ctx, bufferSize, handleFn),
	}
}

func NewFireAndForgetPartitionableHandler[P effectmodel.Partitionable](
	ctx context.Context,
	effectId string,
	numWorkers int,
	bufferSize int,
	handleFn func(context.Context, P),
) FireAndForgetHandler[P] {
	return FireAndForgetHandler[P]{
		EffectId:   effectId,
		dispatcher: NewPartitionedQueue(ctx, numWorkers, bufferSize, handleFn),
	}
}

func (ffh FireAndForgetHandler[P]) FireAndForgetEffect(ctx context.Context, payload P) {
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
