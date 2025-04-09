package handlers

import (
	"context"
	"log"

	"github.com/google/uuid"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

func NewFireAndForgetPartitionableEffectHandler[T effectmodel.Partitionable](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, T),
	teardown func(),
) FireAndForgetPartitionableEffectHandler[T] {
	return FireAndForgetPartitionableEffectHandler[T]{
		fireAndForgetPartitionableEffectScope: newFireAndForgetPartitionableEffectScope(ctx, config, handleFn, teardown),
	}
}

type FireAndForgetPartitionableEffectHandler[T effectmodel.Partitionable] struct {
	*fireAndForgetPartitionableEffectScope[T]
}

func (ffpeh FireAndForgetPartitionableEffectHandler[T]) FireAndForgetPartitionableEffect(ctx context.Context, payload T) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf(
				"panic while sending to closed channel for effect: %+v",
				map[string]interface{}{
					"effectId": ffpeh.EffectId,
					"payload":  payload,
				},
			)
		}
	}()

	msg := fireAndForgetPartitionableEffectMessage[T]{
		payload: payload,
	}
	select {
	case <-ctx.Done():
	case ffpeh.effectChs[getIndexByHash(msg.payload, len(ffpeh.effectChs))] <- msg:
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
type fireAndForgetPartitionableEffectScope[T effectmodel.Partitionable] struct {
	EffectId  string
	effectChs []chan fireAndForgetPartitionableEffectMessage[T]
	closeFn   func()
	closed    bool
}

func (ffpes *fireAndForgetPartitionableEffectScope[T]) Close() {
	if !ffpes.closed {
		ffpes.closeFn()
		ffpes.closed = true
		log.Printf("Effect scope closed: %s", ffpes.EffectId)
	}
}

func newFireAndForgetPartitionableEffectScope[T effectmodel.Partitionable](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, T),
	teardown func(),
) *fireAndForgetPartitionableEffectScope[T] {
	ctx, cancelFn := context.WithCancel(ctx)
	effChs := make([]chan fireAndForgetPartitionableEffectMessage[T], config.NumWorkers)
	for i := 0; i < config.NumWorkers; i++ {
		effCh := make(chan fireAndForgetPartitionableEffectMessage[T], config.BufferSize)
		go func(ch chan fireAndForgetPartitionableEffectMessage[T]) {
			defer close(ch)
			for {
				select {
				case msg := <-ch:
					handleFn(ctx, msg.payload)
				case <-ctx.Done():
					return
				}
			}
		}(effCh)
		effChs[i] = effCh
	}

	return &fireAndForgetPartitionableEffectScope[T]{
		EffectId:  uuid.New().String(),
		effectChs: effChs,
		closeFn: func() {
			cancelFn()
			teardown()
		},
		closed: false,
	}
}

type fireAndForgetPartitionableEffectMessage[T effectmodel.Partitionable] struct {
	payload T
}
