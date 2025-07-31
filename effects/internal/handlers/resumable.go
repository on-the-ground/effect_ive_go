package handlers

import (
	"context"
	"log"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

type ResumableHandler[P any] struct {
	EffectId   string
	dispatcher WorkerDispatcher[ResumableEffectMessage[P]]
}

func NewResumableHandler[P any](
	ctx context.Context,
	effectId string,
	bufferSize int,
	handleFn func(context.Context, P) (any, error),
) ResumableHandler[P] {
	return ResumableHandler[P]{
		EffectId: effectId,
		dispatcher: NewSingleQueue(ctx, bufferSize,
			func(ctx context.Context, msg ResumableEffectMessage[P]) {
				select {
				case <-ctx.Done():
				case msg.ResumeCh <- ResumableResultFrom(handleFn(ctx, msg.Payload)):
				}
				close(msg.ResumeCh)
			},
		),
	}
}

func NewResumablePartitionableHandler[P effectmodel.Partitionable](
	ctx context.Context,
	effectId string,
	numWorkers int,
	bufferSize int,
	handleFn func(context.Context, P) (any, error),
) ResumableHandler[P] {
	return ResumableHandler[P]{
		EffectId: effectId,
		dispatcher: NewPartitionedQueue(ctx, numWorkers, bufferSize,
			func(ctx context.Context, msg ResumableEffectMessage[P]) {
				select {
				case <-ctx.Done():
				case msg.ResumeCh <- ResumableResultFrom(handleFn(ctx, msg.Payload)):
				}
				close(msg.ResumeCh)
			},
		),
	}
}

func (rh ResumableHandler[P]) PerformEffect(ctx context.Context, payload P) <-chan ResumableResult {
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
	resumeCh := make(chan ResumableResult, 1)

	msg := ResumableEffectMessage[P]{
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
type ResumableResult struct {
	Value any
	Err   error
}

func ResumableResultFrom(res any, err error) ResumableResult {
	return ResumableResult{Value: res, Err: err}
}

var _ effectmodel.Partitionable = ResumableEffectMessage[effectmodel.Partitionable]{}

type ResumableEffectMessage[P any] struct {
	Payload  P
	ResumeCh chan ResumableResult
}

func (rem ResumableEffectMessage[P]) PartitionKey() string {
	if p, ok := any(rem.Payload).(effectmodel.Partitionable); ok {
		return p.PartitionKey()
	}
	return ""
}
