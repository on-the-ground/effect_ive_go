module github.com/on-the-ground/effect_ive_go/examples/singed_poison_trail

go 1.24.2

require github.com/on-the-ground/effect_ive_go v0.0.1

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/govalues/decimal v0.1.36 // indirect
	github.com/rickb777/date/v2 v2.1.8 // indirect
	github.com/rickb777/period v1.0.9 // indirect
	github.com/rickb777/plural v1.4.2 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
)

replace (
    github.com/on-the-ground/effect_ive_go v0.0.1 => ../..
)