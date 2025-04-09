package effectmodel

type EffectEnum string

const (
	EffectLog         EffectEnum = "effect_ive_go_effect_enum_log"
	EffectConcurrency EffectEnum = "effect_ive_go_effect_enum_concurrency"
	EffectState       EffectEnum = "effect_ive_go_effect_enum_state"
	EffectBinding     EffectEnum = "effect_ive_go_effect_enum_binding"
	ParentContextKey             = "effect_ive_go_parent_context_key"
)

type EffectScopeConfig struct {
	BufferSize int // default: 1
	NumWorkers int // default: 1 (for fan-out processing if needed in future)
}

func NewEffectScopeConfig(bufferSize int, numWorkers int) EffectScopeConfig {
	if bufferSize <= 0 {
		bufferSize = 1
	}
	if numWorkers <= 0 {
		numWorkers = 1
	}
	return EffectScopeConfig{
		BufferSize: bufferSize,
		NumWorkers: numWorkers,
	}
}

type Partitionable interface {
	PartitionKey() string
}
