package handlers

import (
	"context"
	"log"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

func NewResumableEffectHandler[P any, R any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, ResumableEffectMessage[P, R]),
	teardown func(),
) ResumableHandler[P, R] {
	return ResumableHandler[P, R]{
		effectScope: newEffectScope(
			NewSingleQueue(ctx, config.BufferSize, handleFn),
			teardown,
		),
	}
}

func NewPartitionableResumableHandler[P effectmodel.Partitionable, R any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, ResumableEffectMessage[P, R]),
	teardown func(),
) ResumableHandler[P, R] {
	ctx, cancelFn := context.WithCancel(ctx)
	return ResumableHandler[P, R]{
		effectScope: newEffectScope(
			NewPartitionedQueue(ctx, config.NumWorkers, config.BufferSize, handleFn),
			func() {
				cancelFn()
				teardown()
			},
		),
	}
}

type ResumableHandler[P any, R any] struct {
	*effectScope[ResumableEffectMessage[P, R]]
}

func (rh ResumableHandler[P, R]) PerformEffect(ctx context.Context, payload P) R {
	defer func() {
		if r := recover(); r != nil {
			log.Printf(
				"panic while sending to closed channel for effect: %+v",
				map[string]interface{}{
					"effectId": rh.EffectId,
					"payload":  payload,
				},
			)
		}
	}()

	// buffered to prevent blocking if handler sends without waiting
	resumeCh := make(chan R, 1)

	msg := ResumableEffectMessage[P, R]{
		Payload:  payload,
		ResumeCh: resumeCh,
	}
	select {
	case <-ctx.Done():
	case rh.dispatcher.GetChannelOf(msg) <- msg:
	}

	select {
	case ret := <-resumeCh:
		return ret
	case <-ctx.Done():
	}
	return *new(R)
}

var _ effectmodel.Partitionable = ResumableEffectMessage[any, any]{}

type ResumableEffectMessage[P any, R any] struct {
	Payload  P
	ResumeCh chan R
}

func (rem ResumableEffectMessage[P, R]) PartitionKey() string {
	if p, ok := any(rem.Payload).(effectmodel.Partitionable); ok {
		return p.PartitionKey()
	}
	return ""
}
