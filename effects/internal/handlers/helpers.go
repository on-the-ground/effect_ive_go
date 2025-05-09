package handlers

import (
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"

	"github.com/cespare/xxhash/v2"
)

func hash(key string) uint64 {
	return xxhash.Sum64String(key)
}

func getIndexByHash(payload effectmodel.Partitionable, numChs int) int {
	switch numChs {
	case 0:
		panic("number of channels cannot be 0")
	case 1:
		return 0
	default:
		return int(hash(payload.PartitionKey()) % uint64(numChs))
	}
}
