package effects_test

import (
	"context"

	"github.com/on-the-ground/effect_ive_go/effects"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func WithTestLogEffectHandler(
	ctx context.Context,
) (context.Context, func() context.Context) {
	core, _ := observer.New(zap.DebugLevel)
	return effects.WithZapLogEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		zap.New(core),
	)
}
