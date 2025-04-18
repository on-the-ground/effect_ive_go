package effects_test

import (
	"context"
	"os"

	"github.com/on-the-ground/effect_ive_go/effects"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func WithTestLogEffectHandler(
	ctx context.Context,
) (context.Context, func() context.Context) {
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.Lock(os.Stdout),
		zap.DebugLevel,
	)
	return effects.WithZapLogEffectHandler(
		ctx,
		1,
		zap.New(consoleCore),
	)
}
