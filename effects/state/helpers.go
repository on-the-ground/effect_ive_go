package state

import (
	"context"

	"github.com/on-the-ground/effect_ive_go/effects/internal/helper"
)

// GetFromStateEffect fetches a typed value from the State effect using the provided key.
// Returns a zero value and error if the key is not found or the type is mismatched.
func GetFromStateEffect[T any](ctx context.Context, key string) (T, error) {
	return helper.GetTypedValueOf[T](func() (any, error) {
		return StateEffect(ctx, LoadStatePayload{Key: key})
	})
}

// MustGetFromStateEffect is the panic-on-failure variant of GetFromStateEffect.
// It panics if the key is missing or the type doesn't match.
func MustGetFromStateEffect[T any](ctx context.Context, key string) T {
	return helper.MustGetTypedValue[T](func() (any, error) {
		return StateEffect(ctx, LoadStatePayload{Key: key})
	})
}
