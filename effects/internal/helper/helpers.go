package helper

import (
	"context"
	"fmt"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

// GetHandler checks whether a handler for the given EffectEnum is registered in the context.
// Returns an error if not found.
func GetHandler(ctx context.Context, enum effectmodel.EffectEnum) (any, error) {
	raw := ctx.Value(enum)
	if raw == nil {
		return nil, fmt.Errorf("%w: %v", effectmodel.ErrNoEffectHandler, enum)
	}
	return raw, nil
}

// GetTypedValueOf safely asserts the result of a getter function to the expected type T.
// Returns an error if type assertion fails.
func GetTypedValueOf[T any](getFn func() (any, error)) (T, error) {
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

// GetTypedValueOf2 safely asserts the result of a getter function to the expected type T.
// Returns an error if type assertion fails.
func GetTypedValueOf2[T any](getFn func() (any, bool)) (res T, ok bool) {
	var raw any
	if raw, ok = getFn(); ok {
		res, ok = raw.(T)
	}
	return
}

// MustGetTypedValue is the panic-on-failure variant of getTypedValue.
// Use when failure should be fatal (e.g., when effect handler is guaranteed to exist).
func MustGetTypedValue[T any](getFn func() (any, error)) T {
	res, err := GetTypedValueOf[T](getFn)
	if err != nil {
		panic(err)
	}
	return res
}

var ErrMaxAttempts = fmt.Errorf("max attempts reached")

func Retry(maxAttemps int, fn func() error) error {
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
