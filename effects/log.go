package effects

import (
	"context"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	"go.uber.org/zap"
)

type LogLevel string

const (
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
	LogDebug LogLevel = "debug"
)

type LogPayload struct {
	Level   LogLevel
	Message string
	Fields  map[string]interface{}
}

func WithZapLogEffectHandler(ctx context.Context, logger *zap.Logger) (context.Context, func()) {
	bufferSize, err := GetFromBindingEffect[int](ctx, "config.effect.binding.handler.buffer_size")
	if err != nil {
		bufferSize = 1
	}
	numWorkers, err := GetFromBindingEffect[int](ctx, "config.effect.binding.handler.num_workers")
	if err != nil {
		numWorkers = 1
	}
	return WithFireAndForgetEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(bufferSize, numWorkers),
		effectmodel.EffectLog,
		func(ctx context.Context, payload LogPayload) {
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
			case LogDebug:
				logger.Debug(payload.Message, fields...)
			default:
				logger.Info(payload.Message, fields...)
			}
		},
	)
}

func LogEffect(ctx context.Context, level LogLevel, msg string, fields map[string]interface{}) {
	FireAndForgetEffect(ctx, effectmodel.EffectLog, LogPayload{
		Level:   level,
		Message: msg,
		Fields:  fields,
	})
}
