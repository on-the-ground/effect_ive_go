package state

import (
	"context"

	"github.com/on-the-ground/effect_ive_go/effects/internal/helper"
)

// GetFromStateEff fetches a typed value from the State effect using the provided key.
// Returns a zero value and error if the key is not found or the type is mismatched.
func GetFromStateEff[T any](ctx context.Context, key string) (T, error) {
	return helper.GetTypedValueOf[T](func() (any, error) {
		return StateEff(ctx, LoadStatePayload{Key: key})
	})
}

// MustGetFromStateEff is the panic-on-failure variant of GetFromStateEffect.
// It panics if the key is missing or the type doesn't match.
func MustGetFromStateEff[T any](ctx context.Context, key string) T {
	return helper.MustGetTypedValue[T](func() (any, error) {
		return StateEff(ctx, LoadStatePayload{Key: key})
	})
}
