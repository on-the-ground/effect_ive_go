package effects

import (
	"context"
)

// this is safe only in single goroutine – NEVER share across goroutines
type effectScope[T any, R any] struct {
	effCh    chan effectMessage[T, R]
	teardown func()
	closed   bool
}

func (es *effectScope[T, R]) close() {
	if !es.closed {
		es.teardown()
		// log.
		es.closed = true
	}
}

func newEffectScope[T any, R any](ctx context.Context, handler func(T) R, teardown func()) *effectScope[T, R] {
	effCh := make(chan effectMessage[T, R], 1)

	go func() {
		defer close(effCh)
		for {
			select {
			case msg := <-effCh:
				msg.resume <- handler(msg.payload)
			case <-ctx.Done():
				return
			}
		}
	}()

	return &effectScope[T, R]{
		effCh:    effCh,
		teardown: teardown,
		closed:   false,
	}
}
