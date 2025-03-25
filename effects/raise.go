package effects

import "context"

func WithRaiseEffect(ctx context.Context, handler func(error)) (context.Context, func()) {
	ctxWithRaiseEffect, endOfRaise := withEffectTyped(ctx, EffectRaise, func(err error) struct{} {
		handler(err)
		return struct{}{}
	})
	return ctxWithRaiseEffect, func() {
		recover()
		endOfRaise()
	}
}

func RaiseEffect(ctx context.Context, err error) {
	performEffect[error, any](ctx, EffectRaise, err)
	panic(err)
}

func RaiseIfErr[T any](ctx context.Context, fn func() (T, error)) T {
	res, err := fn()
	if err != nil {
		RaiseEffect(ctx, err)
	}
	return res
}

func RaiseIfErrOnly(ctx context.Context, fn func() error) {
	if err := fn(); err != nil {
		RaiseEffect(ctx, err)
	}
}
