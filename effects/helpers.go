package effects

import (
	"context"
	"fmt"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

// GetFromStateEffect fetches a typed value from the State effect using the provided key.
// Returns a zero value and error if the key is not found or the type is mismatched.
func GetFromStateEffect[T any](ctx context.Context, key string) (T, error) {
	return getTypedValue[T](func() (any, error) {
		return StateEffect(ctx, GetStatePayload{Key: key})
	})
}

// MustGetFromStateEffect is the panic-on-failure variant of GetFromStateEffect.
// It panics if the key is missing or the type doesn't match.
func MustGetFromStateEffect[T any](ctx context.Context, key string) T {
	return mustGetTypedValue[T](func() (any, error) {
		return StateEffect(ctx, GetStatePayload{Key: key})
	})
}

// GetFromBindingEffect fetches a typed value from the Binding effect using the provided key.
// Returns a zero value and error if the key is not found or the type is mismatched.
func GetFromBindingEffect[T any](ctx context.Context, key string) (T, error) {
	return getTypedValue[T](func() (any, error) {
		return BindingEffect(ctx, BindingPayload{Key: key})
	})
}

// MustGetFromBindingEffect is the panic-on-failure variant of GetFromBindingEffect.
// It panics if the key is missing or the type doesn't match.
func MustGetFromBindingEffect[T any](ctx context.Context, key string) T {
	return mustGetTypedValue[T](func() (any, error) {
		return BindingEffect(ctx, BindingPayload{Key: key})
	})
}

// hasHandler checks whether a handler for the given EffectEnum is registered in the context.
// Returns an error if not found.
func hasHandler(ctx context.Context, enum effectmodel.EffectEnum) (any, error) {
	raw := ctx.Value(enum)
	if raw == nil {
		return nil, fmt.Errorf("missing effect handler for %v", enum)
	}
	return raw, nil
}

// getTypedValue safely asserts the result of a getter function to the expected type T.
// Returns an error if type assertion fails.
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

// mustGetTypedValue is the panic-on-failure variant of getTypedValue.
// Use when failure should be fatal (e.g., when effect handler is guaranteed to exist).
func mustGetTypedValue[T any](getFn func() (any, error)) T {
	res, err := getTypedValue[T](getFn)
	if err != nil {
		panic(err)
	}
	return res
}
