package main

import (
	"github.com/on-the-ground/effect_ive_go/effects/state"

	ristretto "github.com/dgraph-io/ristretto/v2"
)

type key interface {
	ristretto.Key
	comparable
}

func NewRistretto[K key](cacheSize int) (state.SetStore[K], error) {
	cache, err := ristretto.NewCache(&ristretto.Config[K, any]{
		NumCounters: 1e7,              // number of keys to track frequency of (10M).
		MaxCost:     1 << 30,          // maximum cost of cache (1GB).
		BufferItems: int64(cacheSize), // number of keys per Get buffer.
	})
	if err != nil {
		return nil, err
	}
	return Ristretto[K]{Cache: cache}, nil
}

type Ristretto[K key] struct {
	*ristretto.Cache[K, any]
}

func (r Ristretto[K]) Get(key K) (v any, ok bool, err error) {
	v, ok = r.Cache.Get(key)
	return
}

func (r Ristretto[K]) Delete(key K) error {
	r.Cache.Del(key)
	return nil
}

func (r Ristretto[K]) Set(key K, value any) error {
	cost := int64(1 << 15)
	r.Cache.Set(key, value, cost)
	return nil
}
