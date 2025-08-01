package dependency_test

import (
	"context"
	"errors"
	"testing"

	"github.com/on-the-ground/effect_ive_go/effects/dependency"
	"github.com/stretchr/testify/require"
)

type Dep1 struct{}

func (Dep1) Id() string { return "dep1" }
func (Dep1) Fn1(ctx context.Context) (string, error) {
	return "result from dep1", nil
}

type Dep2 struct{}

func (*Dep2) Id() string { return "dep2" }

type DepINeed interface {
	Id() string
	Fn1(ctx context.Context) (string, error)
}

type idGetter struct {
	DepINeed
	prefix string
}

func (i idGetter) WithReceiver(dependency any) dependency.Quacker {
	i.DepINeed = dependency.(DepINeed)
	return i
}

func (i idGetter) Quack(ctx context.Context) (any, error) {
	if i.DepINeed == nil {
		return nil, errors.New("receiver not set in Quacker")
	}
	return i.prefix + i.DepINeed.Id(), nil
}

func newIdGetter(prefix string) idGetter {
	return idGetter{prefix: prefix}
}

func TestDependencyEffect_Success(t *testing.T) {
	ctx := context.Background()

	ctx, endDep := dependency.WithEffectHandler(ctx, 1, []any{&Dep2{}, Dep1{}})
	defer endDep()

	ch := dependency.Effect[DepINeed](ctx, newIdGetter("test"))
	res := <-ch

	require.NoError(t, res.Err)
	require.Equal(t, "testdep1", res.Value)
}

func TestDependencyEffect_NoMatchingDependency_NoUpstream(t *testing.T) {
	ctx := context.Background()

	ctx, endDep := dependency.WithEffectHandler(ctx, 1, []any{&Dep2{}})
	defer endDep()

	ch := dependency.Effect[DepINeed](ctx, newIdGetter("oops"))
	res := <-ch

	require.Error(t, res.Err)
	require.Nil(t, res.Value)
}

type Dep3 struct{}

func (Dep3) Id() string { return "dep3" }
func (Dep3) Fn1(ctx context.Context) (string, error) {
	return "dep3 wins", nil
}

func TestDependencyEffect_FirstMatchingWins(t *testing.T) {
	ctx := context.Background()

	ctx, endDep := dependency.WithEffectHandler(ctx, 1, []any{Dep3{}, Dep1{}})
	defer endDep()

	ch := dependency.Effect[DepINeed](ctx, newIdGetter("x"))
	res := <-ch

	require.NoError(t, res.Err)
	require.Equal(t, "xdep3", res.Value)
}

func TestDependencyEffect_QuackWithoutReceiverFails(t *testing.T) {
	q := newIdGetter("z")
	val, err := q.Quack(context.Background())

	require.Error(t, err)
	require.Nil(t, val)
}

func TestDependencyEffect_DelegatesToUpstreamHandler(t *testing.T) {
	ctx := context.Background()

	// uppper handler has a dependency
	ctx, endParent := dependency.WithEffectHandler(ctx, 1, []any{Dep1{}})
	defer endParent()

	// lower handler does not have a dependency
	ctx, endChild := dependency.WithEffectHandler(ctx, 1, []any{})
	defer endChild()

	ch := dependency.Effect[DepINeed](ctx, newIdGetter("top"))
	res := <-ch

	require.NoError(t, res.Err)
	require.Equal(t, "topdep1", res.Value)
}

func TestDependencyEffect_PointerTypeMatch(t *testing.T) {
	// provide a pointer to the dependency
	ctx := context.Background()

	ctx, endDep := dependency.WithEffectHandler(ctx, 1, []any{&Dep1{}})
	defer endDep()

	ch := dependency.Effect[DepINeed](ctx, newIdGetter("p:"))
	res := <-ch

	require.NoError(t, res.Err)
	require.Equal(t, "p:dep1", res.Value)
}
