package effects

type EffectEnum string

const effectIveGoPrefix = "effect_ive_go"
const effectEnumPrefix = "effect_enum"

const (
	EffectLog     EffectEnum = "effect_ive_go_effect_enum_log"
	EffectRaise   EffectEnum = "effect_ive_go_effect_enum_raise"
	EffectTimeout EffectEnum = "effect_ive_go_effect_enum_timeout"
)

type effectMessage[T any, R any] struct {
	payload T
	resume  chan R
}
