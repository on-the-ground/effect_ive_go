package log

import (
	"context"
	"fmt"
	"log"

	"github.com/on-the-ground/effect_ive_go/effects"
)

// LogLevel defines the severity level for log messages.
type LogLevel string

const (
	// LogTrace is used for traceging messages with detailed internal information.
	LogTrace LogLevel = "trace"

	// LogDebug is used for debugging messages with detailed internal information.
	LogDebug LogLevel = "debug"

	// LogInfo is used for general informational messages.
	LogInfo LogLevel = "info"

	// LogWarn is used for potentially harmful situations.
	LogWarn LogLevel = "warn"

	// LogError is used for error events that might still allow the application to continue running.
	LogError LogLevel = "error"

	// LogFatal is used for fatal log messages that terminate the application.
	LogFatal LogLevel = "fatal"

	// LogPanic is used for panic-level log messages that trigger a panic.
	LogPanic LogLevel = "panic"
)

const effectKey effects.EffectKey = "github.com/on-the-ground/effect_ive_go/effects/log/effectKey"

// payload is the payload structure for logging effect.
// It contains the log level, message string, and optional structured fields.
type payload struct {
	Level   LogLevel
	Message string
	Fields  map[string]interface{}
}

func (payload) PartitionKey() string {
	return "unpartitioned"
}

type Logger interface {
	Info(msg string, fields ...map[string]interface{})
	Warn(msg string, fields ...map[string]interface{})
	Error(msg string, fields ...map[string]interface{})
	Debug(msg string, fields ...map[string]interface{})
	Trace(msg string, fields ...map[string]interface{})
	Fatal(msg string, fields ...map[string]interface{})
	Panic(msg string, fields ...map[string]interface{})
	Sync() error
}

// WithEffectHandler registers a fire-and-forget log effect handler using Logger interface.
// It reads buffer size and worker count from the binding effect configuration.
// The returned context includes the handler under the EffectLog effectKey.
// The teardown function should be called when the effect handler is no longer needed.
// If the teardown function is called early, the effect handler will be closed.
// The context returned by the teardown function should be used for further operations.
func WithEffectHandler(
	ctx context.Context,
	bufferSize int,
	logger Logger,
) (context.Context, func() context.Context) {
	return effects.WithFireAndForgetEffectHandler(
		ctx,
		bufferSize,
		effectKey,
		func(ctx context.Context, payload payload) {
			switch payload.Level {
			case LogInfo:
				logger.Info(payload.Message, payload.Fields)
			case LogWarn:
				logger.Warn(payload.Message, payload.Fields)
			case LogError:
				logger.Error(payload.Message, payload.Fields)
			case LogDebug:
				logger.Debug(payload.Message, payload.Fields)
			case LogTrace:
				logger.Trace(payload.Message, payload.Fields)
			case LogFatal:
				logger.Fatal(payload.Message, payload.Fields)
			case LogPanic:
				logger.Panic(payload.Message, payload.Fields)
			default:
				logger.Info(payload.Message, payload.Fields)
			}
		},
		func() {
			if err := logger.Sync(); err != nil {
				logger.Warn(fmt.Sprintf("failed to sync logger: %+v", err))
			}
		},
	)
}

// Effect performs a fire-and-forget log effect using the EffectLog handler in the context.
// This should be used to emit structured logs within an effect-managed execution scope.
func Effect(ctx context.Context, level LogLevel, msg string, fields map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[%s] %s: %+v\n", level, msg, fields)
		}
	}()
	effects.FireAndForgetEffect(ctx, effectKey, payload{
		Level:   level,
		Message: msg,
		Fields:  fields,
	})
}
