package effects

import (
	"context"
	"fmt"
)

type EffectId int

const (
	EffectLog EffectId = iota
	EffectRaise
	EffectTimeout
)

func (e EffectId) String() string {
	switch e {
	case EffectLog:
		return "log"
	case EffectRaise:
		return "raise"
	case EffectTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

type effectKey struct {
	id EffectId
}

type effectMessage[T any, U any] struct {
	payload T
	resume  chan U
}

func withEffectTyped[T any, U any](
	ctx context.Context,
	id EffectId,
	handler func(T) U,
) (context.Context, func()) {
	inbox := make(chan effectMessage[T, U])
	done := make(chan struct{})

	go func() {
		for {
			select {
			case msg := <-inbox:
				msg.resume <- handler(msg.payload)
			case <-done:
				close(inbox)
				return
			}
		}
	}()

	ctxWith := context.WithValue(ctx, effectKey{id: id}, inbox)

	teardown := func() {
		close(done)
	}

	return ctxWith, teardown
}

func performEffect[T any, U any](ctx context.Context, id EffectId, payload T) U {
	key := effectKey{id: id}
	raw := ctx.Value(key)
	if raw == nil {
		panic(fmt.Sprintf("unhandled effect: %s", id))
	}

	inbox, ok := raw.(chan effectMessage[T, U])
	if !ok {
		panic(fmt.Sprintf("invalid handler type for effect: %s (expected chan effectMessage[T, U])", id))
	}

	resume := make(chan U)
	inbox <- effectMessage[T, U]{
		payload: payload,
		resume:  resume,
	}
	return <-resume
}
