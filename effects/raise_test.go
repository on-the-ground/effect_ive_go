package effects_test

import (
	"context"
	"errors"
	"testing"

	"github.com/on-the-ground/effect_ive_go/effects"
)

func TestRaiseEffect_CallsHandlerAndPanics(t *testing.T) {
	ctx := context.Background()
	called := false
	var handledErr error

	ctxWithRaise, done := effects.WithRaiseEffect(ctx, func(err error) {
		called = true
		handledErr = err
	})
	defer func() {
		if !called {
			t.Errorf("Expected handler to be called, but it was not")
		}
		if handledErr == nil || handledErr.Error() != "boom" {
			t.Errorf("Handler received wrong error: %v", handledErr)
		}
	}()
	defer done()

	effects.RaiseEffect(ctxWithRaise, errors.New("boom"))
}

func TestRaiseIfErr_RaisesWhenError(t *testing.T) {
	ctx := context.Background()
	called := false

	ctxWithRaise, done := effects.WithRaiseEffect(ctx, func(err error) {
		called = true
	})
	defer func() {
		if !called {
			t.Errorf("Expected handler to be called, but it was not")
		}
	}()
	defer done()

	_ = effects.RaiseIfErr(ctxWithRaise, func() (string, error) {
		return "", errors.New("oops")
	})
}

func TestRaiseIfErr_DoesNotRaiseWhenNoError(t *testing.T) {
	ctx := context.Background()
	called := false

	ctxWithRaise, done := effects.WithRaiseEffect(ctx, func(err error) {
		called = true
	})
	defer done()

	result := effects.RaiseIfErr(ctxWithRaise, func() (string, error) {
		return "ok", nil
	})

	if result != "ok" {
		t.Errorf("Expected 'ok', got %v", result)
	}
	if called {
		t.Errorf("Handler should not have been called")
	}
}

func TestRaiseIfErrOnly(t *testing.T) {
	ctx := context.Background()
	called := false

	ctxWithRaise, done := effects.WithRaiseEffect(ctx, func(err error) {
		called = true
	})
	defer func() {
		if !called {
			t.Errorf("Expected handler to be called, but it was not")
		}
	}()
	defer done()

	effects.RaiseIfErrOnly(ctxWithRaise, func() error {
		return errors.New("some error")
	})
}

func TestRaiseEffect_PropagatesThroughDeepStack(t *testing.T) {
	ctx := context.Background()
	var called bool
	var receivedErr error
	var calledAfterEffect bool

	level2 := func(ctx context.Context) {
		effects.RaiseEffect(ctx, errors.New("deep error"))
	}

	level1 := func(ctx context.Context) {
		level2(ctx)
		calledAfterEffect = true
	}

	ctxWithRaise, done := effects.WithRaiseEffect(ctx, func(err error) {
		called = true
		receivedErr = err
	})
	defer func() {
		if calledAfterEffect {
			t.Errorf("Expected immediate jump to raise handler")
		}
		if !called {
			t.Errorf("Expected handler to be called")
		}
		if receivedErr == nil || receivedErr.Error() != "deep error" {
			t.Errorf("Handler received wrong error: %v", receivedErr)
		}
	}()
	defer done()

	level1(ctxWithRaise)
}
