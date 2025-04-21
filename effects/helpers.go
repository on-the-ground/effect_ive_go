package effects

import (
	"context"
	"fmt"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

// GetFromStateEffect fetches a typed value from the State effect using the provided key.
// Returns a zero value and error if the key is not found or the type is mismatched.
func GetFromStateEffect[T any](ctx context.Context, key string) (T, error) {
	return getTypedValueOf[T](func() (any, error) {
		return StateEffect(ctx, LoadStatePayload{Key: key})
	})
}

// MustGetFromStateEffect is the panic-on-failure variant of GetFromStateEffect.
// It panics if the key is missing or the type doesn't match.
func MustGetFromStateEffect[T any](ctx context.Context, key string) T {
	return mustGetTypedValue[T](func() (any, error) {
		return StateEffect(ctx, LoadStatePayload{Key: key})
	})
}

// GetFromBindingEffect fetches a typed value from the Binding effect using the provided key.
// Returns a zero value and error if the key is not found or the type is mismatched.
func GetFromBindingEffect[T any](ctx context.Context, key string) (T, error) {
	return getTypedValueOf[T](func() (any, error) {
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

var ErrNoEffectHandler = fmt.Errorf("no effect handler registered for this effect")

// getHandler checks whether a handler for the given EffectEnum is registered in the context.
// Returns an error if not found.
func getHandler(ctx context.Context, enum effectmodel.EffectEnum) (any, error) {
	raw := ctx.Value(enum)
	if raw == nil {
		return nil, fmt.Errorf("%w: %v", ErrNoEffectHandler, enum)
	}
	return raw, nil
}

// getTypedValueOf safely asserts the result of a getter function to the expected type T.
// Returns an error if type assertion fails.
func getTypedValueOf[T any](getFn func() (any, error)) (T, error) {
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
	res, err := getTypedValueOf[T](getFn)
	if err != nil {
		panic(err)
	}
	return res
}

var ErrMaxAttempts = fmt.Errorf("max attempts reached")

func retry(maxAttemps int, fn func() error) error {
	numAttemps := 0
	for {
		err := fn()
		if err == nil {
			return nil
		}
		if numAttemps >= maxAttemps {
			return fmt.Errorf("%w: %d, %w", ErrMaxAttempts, numAttemps, err)
		}
	}
}
