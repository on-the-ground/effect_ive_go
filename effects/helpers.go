package effects

import (
	"context"
	"fmt"

	"github.com/on-the-ground/effect_ive_go/shared/helper"
)

// GetHandler checks whether a handler for the given EffectKey is registered in the context.
// Returns an error if not found.
func GetHandler(ctx context.Context, effectKey EffectKey) (any, error) {
	raw := ctx.Value(effectKey)
	if raw == nil {
		return nil, fmt.Errorf("%w: %v", helper.ErrNoEffectHandler, effectKey)
	}
	return raw, nil
}

// normalizeTeardown flattens optional teardown functions into a single callable.
//
// Accepts either 0 or 1 teardown functions. Panics if more than one is passed.
func normalizeTeardown(teardown []func()) func() {
	switch len(teardown) {
	case 1:
		return teardown[0]
	case 0:
		return func() {}
	default:
		panic("normalizeTeardown: only one or zero teardown functions allowed")
	}
}
