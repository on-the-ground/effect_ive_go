package log

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func WithTestEffectHandler(
	ctx context.Context,
) (context.Context, func() context.Context) {
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.Lock(os.Stdout),
		zap.DebugLevel,
	)
	return WithZapEffectHandler(
		ctx,
		1,
		zap.New(consoleCore),
	)
}
