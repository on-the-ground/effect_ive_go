package effects

import (
	"context"

	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

func WithResumableEffectHandler[T effectmodel.Partitionable, R any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	enum effectmodel.EffectEnum,
	handleFn func(context.Context, T) R,
	teardown ...func(),
) (context.Context, func()) {
	td := normalizeTeardown(teardown)
	handler := handlers.NewResumableEffectHandler(ctx, config, handleFn, td)

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

func PerformResumableEffect[T effectmodel.Partitionable, R any](
	ctx context.Context,
	enum effectmodel.EffectEnum,
	payload T,
) R {
	handler := mustGetTypedValue[handlers.ResumableHandler[T, R]](
		func() (any, error) {
			return hasHandler(ctx, enum)
		},
	)
	return handler.PerformEffect(ctx, payload)
}

func WithFireAndForgetEffectHandler[T any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	enum effectmodel.EffectEnum,
	handleFn func(context.Context, T),
	teardown ...func(),
) (context.Context, func()) {
	td := normalizeTeardown(teardown)
	handler := handlers.NewFireAndForgetEffectHandler(ctx, config, handleFn, td)
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

func FireAndForgetEffect[T any](
	ctx context.Context,
	enum effectmodel.EffectEnum,
	payload T,
) {
	handler := mustGetTypedValue[handlers.FireAndForgetHandler[T]](
		func() (any, error) {
			return hasHandler(ctx, enum)
		},
	)
	handler.FireAndForgetEffect(ctx, payload)
}

func normalizeTeardown(teardown []func()) func() {
	switch len(teardown) {
	case 1:
		return teardown[0]
	case 0:
		return func() {}
	default:
		panic("Too many teardown functions")
	}
}
