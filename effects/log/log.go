package log

import (
	"context"

	"github.com/on-the-ground/effect_ive_go/effects"
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

func (lp LogPayload) PartitionKey() string {
	return "unpartitioned"
}

// WithZapLogEffectHandler registers a fire-and-forget log effect handler using zap.Logger.
// It reads buffer size and worker count from the binding effect configuration.
// The returned context includes the handler under the EffectLog enum.
// The teardown function should be called when the effect handler is no longer needed.
// If the teardown function is called early, the effect handler will be closed.
// The context returned by the teardown function should be used for further operations.
func WithZapLogEffectHandler(
	ctx context.Context,
	bufferSize int,
	logger *zap.Logger,
) (context.Context, func() context.Context) {
	return effects.WithFireAndForgetEffectHandler(
		ctx,
		bufferSize,
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
		func() {
			if err := logger.Sync(); err != nil {
				logger.Warn("failed to sync logger", zap.Error(err))
			}
		},
	)
}

// LogEffect performs a fire-and-forget log effect using the EffectLog handler in the context.
// This should be used to emit structured logs within an effect-managed execution scope.
func LogEff(ctx context.Context, level LogLevel, msg string, fields map[string]interface{}) {
	effects.FireAndForgetEffect(ctx, effectmodel.EffectLog, LogPayload{
		Level:   level,
		Message: msg,
		Fields:  fields,
	})
}
