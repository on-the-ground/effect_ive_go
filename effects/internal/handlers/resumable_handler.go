package handlers

import (
	"context"
	"log"

	"github.com/google/uuid"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

func NewResumableEffectHandler[T any, R any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, T) R,
	teardown func(),
) ResumableHandler[T, R] {
	return ResumableHandler[T, R]{
		resumableEffectScope: newResumableEffectScope(ctx, config, handleFn, teardown),
	}
}

type ResumableHandler[T any, R any] struct {
	*resumableEffectScope[T, R]
}

func (rh ResumableHandler[T, R]) PerformEffect(ctx context.Context, payload T) (ret R) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic while sending to closed channel for effect: %+v", map[string]interface{}{
				"effectId": rh.EffectId,
				"payload":  payload,
			})
		}
	}()

	resumeCh := make(chan R, 1)

	select {
	case <-ctx.Done():
	case rh.EffectCh <- resumableEffectMessage[T, R]{
		payload:  payload,
		resumeCh: resumeCh,
	}:
	}

	select {
	case ret = <-resumeCh:
	case <-ctx.Done():
	}
	return
}

// IMPORTANT:
// This effect handler is **intentionally NOT thread-safe**.
//
// It is designed with the assumption that each handler instance will be used
// only within a **single goroutine** and **single execution scope**.
//
// ➤ We deliberately avoided synchronization (mutexes, atomic ops, etc.)
//
//	to keep the handler lightweight and avoid accidental cross-goroutine sharing.
//
// ➤ Sharing this handler or its context across multiple goroutines
//
//	will lead to **undefined behavior**, including data races, panics, or deadlocks.
//
// This is a **conscious design choice** to reinforce proper scoping and ownership.
// If you require shared access, explicitly manage synchronization outside this scope.
type resumableEffectScope[T any, R any] struct {
	EffectId string
	EffectCh chan resumableEffectMessage[T, R]
	closeFn  func()
	closed   bool
}

func (es *resumableEffectScope[T, R]) Close() {
	if !es.closed {
		es.closeFn()
		es.closed = true
	}
}

func newResumableEffectScope[T any, R any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, T) R,
	teardown func(),
) *resumableEffectScope[T, R] {
	ctx, cancelFn := context.WithCancel(ctx)
	effCh := make(chan resumableEffectMessage[T, R], config.BufferSize)

	go func() {
		defer close(effCh)
		for {
			select {
			case msg := <-effCh:
				msg.resumeCh <- handleFn(ctx, msg.payload)
			case <-ctx.Done():
				return
			}
		}
	}()

	return &resumableEffectScope[T, R]{
		EffectId: uuid.New().String(),
		EffectCh: effCh,
		closeFn: func() {
			cancelFn()
			teardown()
		},
		closed: false,
	}
}

type resumableEffectMessage[T any, R any] struct {
	payload  T
	resumeCh chan R
}
