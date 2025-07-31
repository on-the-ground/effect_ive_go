package dependency

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/on-the-ground/effect_ive_go/effects"
	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/shared/helper"
)

// Exported so consumers can refer to the handler key.
const effectKey effects.EffectKey = "github.com/on-the-ground/effect_ive_go/effects/dependency/effectKey"

func WithEffectHandler(
	pctx context.Context,
	bufferSize int,
	dependencies []any,
) (context.Context, func() context.Context) {
	ctx, endOfHandler := effects.WithResumableEffectHandler(
		pctx,
		bufferSize,
		effectKey,
		func(ctx context.Context, pl payload) (any, error) {

			for _, dep := range dependencies {
				if !implements(dep, pl.duck) {
					continue
				}
				log.Effect(ctx, log.LogDebug, "[dependency]", map[string]interface{}{
					"matched":  fmt.Sprintf("%T", dep),
					"expected": pl.duck,
				})
				return pl.quacker.WithReceiver(dep).Quack(ctx)
			}

			res := <-delegateDependencyEffect(ctx, pl)
			return res.Value, res.Err
		},
	)

	return ctx, endOfHandler
}

type Quacker interface {
	WithReceiver(receiver any) Quacker
	Quack(ctx context.Context) (any, error)
}

func Effect[D any](ctx context.Context, quacker Quacker) <-chan handlers.ResumableResult {
	key := reflect.TypeOf((*D)(nil)).Elem()

	pl := payload{
		duck:    key,
		quacker: quacker,
	}

	return effects.PerformResumableEffect(ctx, effectKey, pl)
}

type payload struct {
	duck    reflect.Type
	quacker Quacker
}

func (p payload) PartitionKey() string {
	return fmt.Sprintf("%v", p.duck)
}

func implements(dep any, duck reflect.Type) bool {
	depType := reflect.TypeOf(dep)

	if depType.Kind() != reflect.Ptr {
		// resolve PointerTo once (Go 1.21 이하 호환)
		var ptrTo func(reflect.Type) reflect.Type
		if p := reflect.ValueOf(reflect.PointerTo); p.IsValid() {
			ptrTo = reflect.PointerTo // Go 1.22+
		} else {
			ptrTo = reflect.PtrTo // Go <= 1.21
		}
		depType = ptrTo(depType)
	}

	return depType.Implements(duck)
}

func delegateDependencyEffect(pctx context.Context, pl payload) (ch <-chan handlers.ResumableResult) {
	defer func() {
		if r := recover(); r != nil {
			if r, ok := r.(error); ok && errors.Is(r, helper.ErrNoEffectHandler) {
				_ch := make(chan handlers.ResumableResult, 1)
				_ch <- handlers.ResumableResultFrom(nil, r)
				ch = _ch
			} else {
				panic(r)
			}
		}
	}()

	return effects.PerformResumableEffect(pctx, effectKey, pl)
}
