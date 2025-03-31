package effectmodel

type EffectEnum string

const (
	EffectLog         EffectEnum = "effect_ive_go_effect_enum_log"
	EffectConcurrency EffectEnum = "effect_ive_go_effect_enum_concurrency"
	EffectTimeout     EffectEnum = "effect_ive_go_effect_enum_timeout"
)

type ResumableEffectMessage[T any, R any] struct {
	Payload  T
	ResumeCh chan R
}

type FireAndForgetEffectMessage[T any] struct {
	Payload T
}
