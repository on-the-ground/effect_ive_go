package main

import (
	"context"
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/on-the-ground/effect_ive_go/effects/concurrency"
	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/effects/state"
	"go.uber.org/zap"
)

type Person struct {
	Email string
	Name  string
	Age   int
}

var (
	person1 = Person{
		Email: "email1",
		Name:  "name1",
		Age:   1,
	}
	person2 = Person{
		Email: "email2",
		Name:  "name2",
		Age:   2,
	}
	person3 = Person{
		Email: "email3",
		Name:  "name3",
		Age:   3,
	}
)

func main() {
	ctx := context.Background()
	logger, _ := zap.NewProduction()
	ctx, endOfLog := log.WithZapEffectHandler(ctx, 10, logger)
	defer endOfLog()

	ctx, endOfConcurrency := concurrency.WithEffectHandler(ctx, 10)
	defer endOfConcurrency()

	var schema = &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"person": {
				Name: "person",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Email"},
					},
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(ctx)

	// Tier 1: memDB
	memDB, err := NewMemDBStore[string]("person", "id", schema)
	if err != nil {
		panic(fmt.Sprintf("fail to get memDB store: %v", err))
	}
	ctx, endOfDBHandler := state.WithEffectHandler[string, Person](
		ctx,
		1, 1,
		false,
		state.NewCasStore(memDB),
		nil,
	)
	defer endOfDBHandler()

	dbSource, err := state.EffectSource(ctx)
	if err != nil {
		panic(fmt.Sprintf("fail to get source channel of db: %v", err))
	}
	concurrency.Effect(ctx, watchEventsFrom(dbSource, "db"))

	// Tier 0: cache
	rist, err := NewRistretto[string](2)
	if err != nil {
		panic(fmt.Sprintf("fail to get ristretto cache: %v", err))
	}
	ctx, endOfCacheHandler := state.WithEffectHandler[string, Person](
		ctx,
		1, 1,
		true,
		state.NewSetStore(rist),
		nil,
	)
	defer endOfCacheHandler()

	cacheSource, err := state.EffectSource(ctx)
	if err != nil {
		panic(fmt.Sprintf("fail to get source channel of db: %v", err))
	}
	concurrency.Effect(ctx, watchEventsFrom(cacheSource, "cache"))

	// Insert trials
	insertAndLog(ctx, "person1", person1)
	insertAndLog(ctx, "person1", person2) // should fail due to key conflict
	insertAndLog(ctx, "person2", person2)
	insertAndLog(ctx, "person3", person3)
	insertAndLog(ctx, "person1", person3) // again conflict

	// CAS update
	casAndLog(ctx, "person2", person2, person3)

	// Final state
	for _, key := range []string{"person1", "person2", "person3"} {
		val, err := state.EffectLoad[string, Person](ctx, key)
		if err == nil {
			log.Effect(ctx, log.LogInfo, "final state", map[string]interface{}{
				"key":   key,
				"value": val,
			})
		} else {
			log.Effect(ctx, log.LogInfo, "final state", map[string]interface{}{
				"key":   key,
				"value": val,
			})
		}
	}

	cancel()
}

func watchEventsFrom(ch <-chan state.TimeBoundedPayload, name string) func(ctx context.Context) {
	return func(ctx context.Context) {
		for {
			select {
			case msg := <-ch:
				log.Effect(ctx, log.LogInfo, name+" event", map[string]interface{}{
					"event_type": fmt.Sprintf("%T", msg.Payload),
					"payload":    msg.Payload,
					"timestamp":  msg.TimeSpan.Start,
				})
			case <-ctx.Done():
				return
			}
		}
	}
}

// Insert helper with logging
func insertAndLog(ctx context.Context, key string, person Person) {
	ok, err := state.EffectInsertIfAbsent(ctx, key, person)
	log.Effect(ctx, log.LogInfo, "insert attempt", map[string]interface{}{
		"key":     key,
		"value":   person,
		"success": ok,
		"error":   err,
	})
}

func casAndLog(ctx context.Context, key string, old, new Person) {
	ok, err := state.EffectCompareAndSwap(ctx, key, old, new)
	log.Effect(ctx, log.LogInfo, "cas attempt", map[string]interface{}{
		"key":     key,
		"old":     old,
		"new":     new,
		"success": ok,
		"error":   err,
	})
}
