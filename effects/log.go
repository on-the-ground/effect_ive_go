package effects

import (
	"context"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/model"
	"go.uber.org/zap"
)

type LogLevel string

const (
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
)

type LogPayload struct {
	Level   LogLevel
	Message string
	Fields  map[string]interface{}
}

func WithZapLogEffectHandler(ctx context.Context, logger *zap.Logger) (context.Context, func()) {
	return WithFireAndForgetEffectHandler(ctx, effectmodel.EffectLog, func(ctx context.Context, payload LogPayload) {
		fields := make([]zap.Field, 0, len(payload.Fields))
		for k, v := range payload.Fields {
			fields = append(fields, zap.Any(k, v))
		}

		switch payload.Level {
		case LogInfo:
			logger.Info(payload.Message, fields...)
		case LogWarn:
			logger.Warn(payload.Message, fields...)
		case LogError:
			logger.Error(payload.Message, fields...)
		default:
			logger.Info(payload.Message, fields...)
		}
	})
}

func LogEffect(ctx context.Context, level LogLevel, msg string, fields map[string]interface{}) {
	FireAndForgetEffect(ctx, effectmodel.EffectLog, LogPayload{
		Level:   level,
		Message: msg,
		Fields:  fields,
	})
}
