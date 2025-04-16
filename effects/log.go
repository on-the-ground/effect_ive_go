package effects

import (
	"context"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	"go.uber.org/zap"
)

// LogLevel defines the severity level for log messages.
type LogLevel string

const (
	// LogInfo is used for general informational messages.
	LogInfo LogLevel = "info"

	// LogWarn is used for potentially harmful situations.
	LogWarn LogLevel = "warn"

	// LogError is used for error events that might still allow the application to continue running.
	LogError LogLevel = "error"

	// LogDebug is used for debugging messages with detailed internal information.
	LogDebug LogLevel = "debug"
)

// LogPayload is the payload structure for logging effect.
// It contains the log level, message string, and optional structured fields.
type LogPayload struct {
	Level   LogLevel
	Message string
	Fields  map[string]interface{}
}

// WithZapLogEffectHandler registers a fire-and-forget log effect handler using zap.Logger.
// It reads buffer size and worker count from the binding effect configuration.
// The returned context includes the handler under the EffectLog enum.
func WithZapLogEffectHandler(
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	logger *zap.Logger,
) (context.Context, func() context.Context) {
	return WithFireAndForgetEffectHandler(
		ctx,
		config,
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

// LogEffect performs a fire-and-forget log effect using the EffectLog handler in the context.
// This should be used to emit structured logs within an effect-managed execution scope.
func LogEffect(ctx context.Context, level LogLevel, msg string, fields map[string]interface{}) {
	FireAndForgetEffect(ctx, effectmodel.EffectLog, LogPayload{
		Level:   level,
		Message: msg,
		Fields:  fields,
	})
}
