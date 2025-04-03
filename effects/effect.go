package effects

import (
	"context"
	"fmt"
)

func WithEffect[T any, R any](
	ctx context.Context,
	enum EffectEnum,
	handler func(T) R,
	teardown ...func(),
) (context.Context, func()) {
	var td func()
	switch len(teardown) {
	case 1:
		td = teardown[0]
	case 0:
		td = func() {}
	default:
		panic("Too many teardown functions")
	}

	scope := newEffectScope(ctx, handler, td)
	ctxWith := context.WithValue(ctx, enum, scope)
	return ctxWith, scope.close
}

func PerformEffect[T any, R any](ctx context.Context, enum EffectEnum, payload T) R {
	raw := ctx.Value(enum)
	if raw == nil {
		panic(fmt.Sprintf("no handler for effect: %s", enum))
	}
	scope, ok := raw.(*effectScope[T, R])
	if !ok {
		panic(fmt.Sprintf("no handler for effect: %s", enum))

	}
	resume := make(chan R, 1)
	scope.effCh <- effectMessage[T, R]{
		payload: payload,
		resume:  resume,
	}
	result := <-resume
	return result
}
