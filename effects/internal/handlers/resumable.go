package handlers

import (
	"context"
	"log"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

type ResumableHandler[P any, R any] struct {
	EffectId   string
	dispatcher WorkerDispatcher[ResumableEffectMessage[P, R]]
}

func NewResumableHandler[P any, R any](
	ctx context.Context,
	effectId string,
	bufferSize int,
	handleFn func(context.Context, P) (R, error),
) ResumableHandler[P, R] {
	return ResumableHandler[P, R]{
		EffectId: effectId,
		dispatcher: NewSingleQueue(ctx, bufferSize,
			func(ctx context.Context, msg ResumableEffectMessage[P, R]) {
				select {
				case <-ctx.Done():
				case msg.ResumeCh <- ResumableResultFrom(handleFn(ctx, msg.Payload)):
				}
				close(msg.ResumeCh)
			},
		),
	}
}

func NewResumablePartitionableHandler[P effectmodel.Partitionable, R any](
	ctx context.Context,
	effectId string,
	numWorkers int,
	bufferSize int,
	handleFn func(context.Context, P) (R, error),
) ResumableHandler[P, R] {
	return ResumableHandler[P, R]{
		EffectId: effectId,
		dispatcher: NewPartitionedQueue(ctx, numWorkers, bufferSize,
			func(ctx context.Context, msg ResumableEffectMessage[P, R]) {
				select {
				case <-ctx.Done():
				case msg.ResumeCh <- ResumableResultFrom(handleFn(ctx, msg.Payload)):
				}
				close(msg.ResumeCh)
			},
		),
	}
}

func (rh ResumableHandler[P, R]) PerformEffect(ctx context.Context, payload P) <-chan ResumableResult[R] {
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
	resumeCh := make(chan ResumableResult[R], 1)

	msg := ResumableEffectMessage[P, R]{
		Payload:  payload,
		ResumeCh: resumeCh,
	}
	select {
	case <-ctx.Done():
	case rh.dispatcher.GetChannelOf(msg) <- msg:
	}

	return resumeCh
}

// ResumableResult represents the result of handled effects.
type ResumableResult[T any] struct {
	Value T
	Err   error
}

func ResumableResultFrom[R any](res R, err error) ResumableResult[R] {
	return ResumableResult[R]{Value: res, Err: err}
}

var _ effectmodel.Partitionable = ResumableEffectMessage[effectmodel.Partitionable, any]{}

type ResumableEffectMessage[P any, R any] struct {
	Payload  P
	ResumeCh chan ResumableResult[R]
}

func (rem ResumableEffectMessage[P, R]) PartitionKey() string {
	if p, ok := any(rem.Payload).(effectmodel.Partitionable); ok {
		return p.PartitionKey()
	}
	return ""
}
