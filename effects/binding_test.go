package effects_test

import (
	"context"
	"strings"
	"testing"

	"github.com/on-the-ground/effect_ive_go/effects"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestBindingEffect_BasicLookup(t *testing.T) {
	ctx := context.Background()

	core, _ := observer.New(zap.DebugLevel)
	ctx, endOfLog := effects.WithZapLogEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		zap.New(core),
	)
	defer endOfLog()

	ctx, closeFn := effects.WithBindingEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		map[string]any{
			"foo": 123,
		},
	)
	defer closeFn()

	v, err := effects.BindingEffect(ctx, effects.BindingPayload{Key: "foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 123 {
		t.Fatalf("expected 123, got %v", v)
	}
}

func TestBindingEffect_KeyNotFound(t *testing.T) {
	ctx := context.Background()
	core, _ := observer.New(zap.DebugLevel)
	ctx, endOfLog := effects.WithZapLogEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		zap.New(core),
	)
	defer endOfLog()
	ctx, closeFn := effects.WithBindingEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		map[string]any{
			"foo": 123,
		},
	)
	defer closeFn()

	_, err := effects.BindingEffect(ctx, effects.BindingPayload{Key: "bar"})
	if err == nil || !strings.Contains(err.Error(), "key not found") {
		t.Fatalf("expected key-not-found error, got: %v", err)
	}
}

func TestBindingEffect_DelegatesToUpperScope(t *testing.T) {
	ctx := context.Background()

	core, _ := observer.New(zap.DebugLevel)
	ctx, endOfLog := effects.WithZapLogEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		zap.New(core),
	)
	defer endOfLog()

	upperCtx, upperClose := effects.WithBindingEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(1, 1),
		map[string]any{
			"upper": "delegated",
		},
	)
	defer upperClose()

	lowerCtx := context.WithValue(upperCtx, "dummy", 1)
	lowerCtx, lowerClose := effects.WithBindingEffectHandler(
		lowerCtx,
		effectmodel.NewEffectScopeConfig(1, 1),
		nil,
	)
	defer lowerClose()

	v, err := effects.BindingEffect(lowerCtx, effects.BindingPayload{Key: "upper"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "delegated" {
		t.Fatalf("expected delegated, got %v", v)
	}
}
