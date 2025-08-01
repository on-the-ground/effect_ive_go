package log

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ZapLogger struct {
	zapLogger *zap.Logger
}

func normalizeZapFields(fields ...map[string]interface{}) []zap.Field {
	if len(fields) == 0 {
		return nil
	}
	var zapFields []zap.Field
	for _, _fields := range fields {
		if _fields == nil {
			continue
		}
		for k, v := range _fields {
			zapFields = append(zapFields, zap.Any(k, v))
		}
	}
	return zapFields
}

func (l *ZapLogger) Info(msg string, fields ...map[string]interface{}) {
	l.zapLogger.Info(msg, normalizeZapFields(fields...)...)
}
func (l *ZapLogger) Debug(msg string, fields ...map[string]interface{}) {
	l.zapLogger.Debug(msg, normalizeZapFields(fields...)...)
}
func (l *ZapLogger) Warn(msg string, fields ...map[string]interface{}) {
	l.zapLogger.Warn(msg, normalizeZapFields(fields...)...)
}
func (l *ZapLogger) Error(msg string, fields ...map[string]interface{}) {
	l.zapLogger.Error(msg, normalizeZapFields(fields...)...)
}
func (l *ZapLogger) Fatal(msg string, fields ...map[string]interface{}) {
	l.zapLogger.Fatal(msg, normalizeZapFields(fields...)...)
}
func (l *ZapLogger) Panic(msg string, fields ...map[string]interface{}) {
	l.zapLogger.Panic(msg, normalizeZapFields(fields...)...)
}
func (l *ZapLogger) Sync() error {
	return l.zapLogger.Sync()
}

func newTestZapLogger() Logger {
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.Lock(os.Stdout),
		zap.DebugLevel,
	)
	return &ZapLogger{
		zapLogger: zap.New(consoleCore),
	}
}

// WithTestEffectHandler installs a test log effect handler.
func WithTestEffectHandler(
	ctx context.Context,
) (context.Context, func() context.Context) {
	return WithEffectHandler(
		ctx,
		1,
		newTestZapLogger(),
	)
}
