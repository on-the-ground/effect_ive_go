package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestFireAndForgetEffectHandler_BasicExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var receivedPayload string
	done := make(chan bool)

	handler := handlers.NewFireAndForgetEffectHandler(
		ctx,
		effectmodel.EffectScopeConfig{BufferSize: 10},
		func(ctx context.Context, msg string) {
			receivedPayload = msg
			done <- true
		},
		func() {}, // no-op teardown
	)
	defer handler.Close()

	handler.FireAndForgetEffect(ctx, "hello")

	select {
	case <-done:
		assert.Equal(t, "hello", receivedPayload)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for handler")
	}

}

func TestFireAndForgetEffectHandler_CancelContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	time.Sleep(100 * time.Millisecond)

	var called bool

	handler := handlers.NewFireAndForgetEffectHandler(
		ctx,
		effectmodel.EffectScopeConfig{BufferSize: 10},
		func(ctx context.Context, msg string) {
			called = true
		},
		func() {},
	)
	defer handler.Close()

	handler.FireAndForgetEffect(ctx, "should-not-send")

	assert.False(t, called, "handler should not have been called")
}
