package effectscope

import (
	"context"

	"github.com/google/uuid"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/model"
)

// this is safe only in single goroutine – NEVER share across goroutines
type ResumableEffectScope[T any, R any] struct {
	EffectId string
	EffectCh chan effectmodel.ResumableEffectMessage[T, R]
	closeFn  func()
	closed   bool
}

func (es *ResumableEffectScope[T, R]) Close() {
	if !es.closed {
		es.closeFn()
		es.closed = true
	}
}

func NewResumableEffectScope[T any, R any](ctx context.Context, handleFn func(context.Context, T) R, teardown func()) *ResumableEffectScope[T, R] {
	ctx, cancelFn := context.WithCancel(ctx)
	numEffCh := 1
	effCh := make(chan effectmodel.ResumableEffectMessage[T, R], numEffCh)

	go func() {
		defer close(effCh)
		for {
			select {
			case msg := <-effCh:
				msg.ResumeCh <- handleFn(ctx, msg.Payload)
			case <-ctx.Done():
				return
			}
		}
	}()

	return &ResumableEffectScope[T, R]{
		EffectId: uuid.New().String(),
		EffectCh: effCh,
		closeFn: func() {
			cancelFn()
			teardown()
		},
		closed: false,
	}
}

type FireAndForgetEffectScope[T any] struct {
	EffectId string
	EffectCh chan effectmodel.FireAndForgetEffectMessage[T]
	closeFn  func()
	closed   bool
}

func (es *FireAndForgetEffectScope[T]) Close() {
	if !es.closed {
		es.closeFn()
		// log.
		es.closed = true
	}
}

func NewFireAndForgetEffectScope[T any](ctx context.Context, handleFn func(context.Context, T), teardown func()) *FireAndForgetEffectScope[T] {
	ctx, cancelFn := context.WithCancel(ctx)
	numEffCh := 1
	effCh := make(chan effectmodel.FireAndForgetEffectMessage[T], numEffCh)

	go func() {
		defer close(effCh)
		for {
			select {
			case msg := <-effCh:
				handleFn(ctx, msg.Payload)
			case <-ctx.Done():
				return
			}
		}
	}()

	return &FireAndForgetEffectScope[T]{
		EffectId: uuid.New().String(),
		EffectCh: effCh,
		closeFn: func() {
			cancelFn()
			teardown()
		},
		closed: false,
	}
}
