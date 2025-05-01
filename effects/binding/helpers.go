package binding

import (
	"context"

	"github.com/on-the-ground/effect_ive_go/effects/internal/helper"
)

// GetFromBindingEff fetches a typed value from the Binding effect using the provided key.
// Returns a zero value and error if the key is not found or the type is mismatched.
func GetFromBindingEff[T any](ctx context.Context, key string) (T, error) {
	return helper.GetTypedValueOf[T](func() (any, error) {
		return Effect(ctx, key)
	})
}

// MustGetFromBindingEff is the panic-on-failure variant of GetFromBindingEffect.
// It panics if the key is missing or the type doesn't match.
func MustGetFromBindingEff[T any](ctx context.Context, key string) T {
	return helper.MustGetTypedValue[T](func() (any, error) {
		return Effect(ctx, key)
	})
}
