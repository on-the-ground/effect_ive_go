package handlers_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	"github.com/stretchr/testify/assert"
)

// mockPartitionable is a mock type that implements the Partitionable interface
type mockPartitionable struct {
	id   string
	hash string
}

func (m mockPartitionable) PartitionKey() string {
	return m.hash
}

func TestFireAndForgetPartitionableEffectHandler_BasicExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var received []string
	done := make(chan struct{})

	handler := handlers.NewFireAndForgetPartitionableEffectHandler(
		ctx,
		effectmodel.EffectScopeConfig{
			BufferSize: 5,
			NumWorkers: 2,
		},
		func(ctx context.Context, msg mockPartitionable) {
			mu.Lock()
			received = append(received, msg.id)
			mu.Unlock()
			if len(received) == 2 {
				close(done)
			}
		},
		func() {},
	)
	defer handler.Close()

	handler.FireAndForgetPartitionableEffect(ctx, mockPartitionable{id: "a", hash: "1"})
	handler.FireAndForgetPartitionableEffect(ctx, mockPartitionable{id: "b", hash: "2"})

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for handler to process all messages")
	}

	mu.Lock()
	assert.Contains(t, received, "a")
	assert.Contains(t, received, "b")
	mu.Unlock()
}

func TestFireAndForgetPartitionableEffectHandler_SameHashGoesToSameWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var received []string
	done := make(chan struct{})

	handler := handlers.NewFireAndForgetPartitionableEffectHandler(
		ctx,
		effectmodel.EffectScopeConfig{
			BufferSize: 5,
			NumWorkers: 3,
		},
		func(ctx context.Context, msg mockPartitionable) {
			mu.Lock()
			defer mu.Unlock()

			received = append(received, msg.id)
			if len(received) == 2 {
				close(done)
			}
		},
		func() {},
	)
	defer handler.Close()

	handler.FireAndForgetPartitionableEffect(ctx, mockPartitionable{id: "first", hash: "same"})
	handler.FireAndForgetPartitionableEffect(ctx, mockPartitionable{id: "second", hash: "same"})

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for messages")
	}

	mu.Lock()
	assert.Equal(t, []string{"first", "second"}, received, "messages with same hash should be processed in order")
	mu.Unlock()
}

func TestFireAndForgetPartitionableEffectHandler_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	time.Sleep(100 * time.Millisecond)

	var called bool
	handler := handlers.NewFireAndForgetPartitionableEffectHandler(
		ctx,
		effectmodel.EffectScopeConfig{
			BufferSize: 5,
			NumWorkers: 2,
		},
		func(ctx context.Context, msg mockPartitionable) {
			called = true
		},
		func() {},
	)
	defer handler.Close()

	handler.FireAndForgetPartitionableEffect(ctx, mockPartitionable{id: "cancelled", hash: "0"})

	assert.False(t, called, "Handler should not have been called after context cancellation")
}
