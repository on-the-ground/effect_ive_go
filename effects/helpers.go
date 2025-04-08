package effects

import (
	"context"
	"fmt"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

func GetFromStateEffect[T any](ctx context.Context, key string) (T, error) {
	return getTypedValue[T](func() (any, error) {
		return StateEffect(ctx, GetStatePayload{Key: key})
	})
}

func MustGetFromStateEffect[T any](ctx context.Context, key string) T {
	return mustGetTypedValue[T](func() (any, error) {
		return StateEffect(ctx, GetStatePayload{Key: key})
	})
}

func GetFromBindingEffect[T any](ctx context.Context, key string) (T, error) {
	return getTypedValue[T](func() (any, error) {
		return BindingEffect(ctx, BindingPayload{Key: key})
	})
}

func MustGetFromBindingEffect[T any](ctx context.Context, key string) T {
	return mustGetTypedValue[T](func() (any, error) {
		return BindingEffect(ctx, BindingPayload{Key: key})
	})
}

func hasHandler(ctx context.Context, enum effectmodel.EffectEnum) (any, error) {
	raw := ctx.Value(enum)
	if raw == nil {
		return nil, fmt.Errorf("missing effect handler for %v", enum)
	}
	return raw, nil
}

func getTypedValue[T any](getFn func() (any, error)) (T, error) {
	var zero T

	res, err := getFn()
	if err != nil {
		return zero, fmt.Errorf("failed to get value: %w", err)
	}

	val, ok := res.(T)
	if !ok {
		return zero, fmt.Errorf("unexpected type: %T", res)
	}

	return val, nil
}

func mustGetTypedValue[T any](getFn func() (any, error)) T {
	res, err := getTypedValue[T](getFn)
	if err != nil {
		panic(err)
	}
	return res
}
