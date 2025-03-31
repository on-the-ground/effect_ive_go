package effects

import (
	"context"
	"fmt"

	"github.com/on-the-ground/effect_ive_go/effects/internal/effectscope"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/model"
)

func WithResumableEffectHandler[T any, R any](
	ctx context.Context,
	enum effectmodel.EffectEnum,
	handleFn func(context.Context, T) R,
	teardown ...func(),
) (context.Context, func()) {
	td := assertTeardown(teardown)
	handler := resumableHandler[T, R]{
		ResumableEffectScope: effectscope.NewResumableEffectScope(ctx, handleFn, td),
	}
	LogEffect(ctx, LogInfo, "created resumable effect handler", map[string]interface{}{
		"effectId": handler.EffectId,
		"enum":     enum,
	})
	ctxWith := context.WithValue(ctx, enum, handler)
	return ctxWith, func() {
		handler.Close()
		LogEffect(ctx, LogInfo, "closed resumable effect handler", map[string]interface{}{
			"effectId": handler.EffectId,
			"enum":     enum,
		})
	}
}

func PerformEffect[T any, R any](ctx context.Context, enum effectmodel.EffectEnum, payload T) R {
	raw := ctx.Value(enum)
	if raw == nil {
		panic(fmt.Errorf("missing effect handler for %v", enum))
	}
	handler, ok := raw.(resumableHandler[T, R])
	if !ok {
		panic(fmt.Errorf("missing effect handler for %v", enum))

	}
	return handler.PerformEffect(ctx, payload)
}

func WithFireAndForgetEffectHandler[T any](
	ctx context.Context,
	enum effectmodel.EffectEnum,
	handleFn func(context.Context, T),
	teardown ...func(),
) (context.Context, func()) {
	td := assertTeardown(teardown)
	handler := fireAndForgetHandler[T]{
		FireAndForgetEffectScope: effectscope.NewFireAndForgetEffectScope(ctx, handleFn, td),
	}
	LogEffect(ctx, LogInfo, "created fire/forget effect handler", map[string]interface{}{
		"effectId": handler.EffectId,
		"enum":     enum,
	})
	ctxWith := context.WithValue(ctx, enum, handler)
	return ctxWith, func() {
		handler.Close()
		LogEffect(ctx, LogInfo, "closed fire/forget effect handler", map[string]interface{}{
			"effectId": handler.EffectId,
			"enum":     enum,
		})
	}
}

func FireAndForgetEffect[T any](ctx context.Context, enum effectmodel.EffectEnum, payload T) {
	raw := ctx.Value(enum)
	if raw == nil {
		panic(fmt.Errorf("missing effect handler for %v", enum))
	}
	handler, ok := raw.(fireAndForgetHandler[T])
	if !ok {
		panic(fmt.Errorf("missing effect handler for %v", enum))

	}
	handler.FireAndForgetEffect(ctx, payload)
}

func assertTeardown(teardown []func()) func() {
	switch len(teardown) {
	case 1:
		return teardown[0]
	case 0:
		return func() {}
	default:
		panic("Too many teardown functions")
	}
}

type resumableHandler[T any, R any] struct {
	*effectscope.ResumableEffectScope[T, R]
}

func (rh resumableHandler[T, R]) PerformEffect(ctx context.Context, payload T) (ret R) {
	defer func() {
		if r := recover(); r != nil {
			LogEffect(ctx, LogError, "panic while sending to closed channel for effect", map[string]interface{}{
				"effectId": rh.EffectId,
				"payload":  payload,
			})
		}
	}()

	resumeCh := make(chan R, 1)

	select {
	case rh.EffectCh <- effectmodel.ResumableEffectMessage[T, R]{
		Payload:  payload,
		ResumeCh: resumeCh,
	}:
	case <-ctx.Done():
		return
	}

	ret = <-resumeCh
	return
}

type fireAndForgetHandler[T any] struct {
	*effectscope.FireAndForgetEffectScope[T]
}

func (ffh fireAndForgetHandler[T]) FireAndForgetEffect(ctx context.Context, payload T) {
	defer func() {
		if r := recover(); r != nil {
			LogEffect(ctx, LogError, "panic while sending to closed channel for effect", map[string]interface{}{
				"effectId": ffh.EffectId,
				"payload":  payload,
			})
		}
	}()

	select {
	case ffh.EffectCh <- effectmodel.FireAndForgetEffectMessage[T]{
		Payload: payload,
	}:
	case <-ctx.Done():
	}
}
