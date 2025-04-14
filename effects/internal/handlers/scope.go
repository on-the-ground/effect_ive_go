package handlers

import (
	"log"

	"github.com/google/uuid"
)

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
type effectScope[T any] struct {
	EffectId   string
	dispatcher WorkerDispatcher[T]
	closeFn    func()
	closed     bool
}

func (es *effectScope[T]) Close() {
	if !es.closed {
		es.closeFn()
		es.closed = true
		log.Printf("Effect scope closed: %s", es.EffectId)
	}
}

func newEffectScope[T any](
	dispatcher WorkerDispatcher[T],
	teardown func(),
) *effectScope[T] {
	return &effectScope[T]{
		EffectId:   uuid.New().String(),
		dispatcher: dispatcher,
		closeFn:    teardown,
		closed:     false,
	}
}
