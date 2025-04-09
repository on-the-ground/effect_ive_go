package handlers

import (
	"context"
	"log"

	"github.com/google/uuid"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

func NewFireAndForgetEffectHandler[T any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, T),
	teardown func(),
) FireAndForgetHandler[T] {
	return FireAndForgetHandler[T]{
		fireAndForgetEffectScope: newFireAndForgetEffectScope(ctx, config, handleFn, teardown),
	}
}

type FireAndForgetHandler[T any] struct {
	*fireAndForgetEffectScope[T]
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
	case ffh.effectCh <- fireAndForgetEffectMessage[T]{
		payload: payload,
	}:
	}
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
type fireAndForgetEffectScope[T any] struct {
	EffectId string
	effectCh chan fireAndForgetEffectMessage[T]
	closeFn  func()
	closed   bool
}

func (ffs *fireAndForgetEffectScope[T]) Close() {
	if !ffs.closed {
		ffs.closeFn()
		// log.
		ffs.closed = true
	}
}

func newFireAndForgetEffectScope[T any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, T),
	teardown func(),
) *fireAndForgetEffectScope[T] {
	ctx, cancelFn := context.WithCancel(ctx)
	effCh := make(chan fireAndForgetEffectMessage[T], config.BufferSize)

	go func() {
		defer close(effCh)
		for {
			select {
			case msg := <-effCh:
				handleFn(ctx, msg.payload)
			case <-ctx.Done():
				return
			}
		}
	}()

	return &fireAndForgetEffectScope[T]{
		EffectId: uuid.New().String(),
		effectCh: effCh,
		closeFn: func() {
			cancelFn()
			teardown()
		},
		closed: false,
	}
}

type fireAndForgetEffectMessage[T any] struct {
	payload T
}
