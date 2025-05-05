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
