package effects

import "context"

type LogMessage struct {
	Text  string
	Level string
}

func WithLogEffect(ctx context.Context, handler func(LogMessage)) (context.Context, func()) {
	return withEffectTyped(ctx, EffectLog, func(msg LogMessage) struct{} {
		handler(msg)
		return struct{}{}
	})
}

func LogEffect(ctx context.Context, payload LogMessage) {
	performEffect[LogMessage, struct{}](ctx, EffectLog, payload)
}
