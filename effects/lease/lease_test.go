package lease_test

import (
	"context"
	"testing"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects/lease"
	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/stretchr/testify/require"
)

func TestLeaseEffect_BasicLifecycle(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, teardown := lease.WithInMemoryEffectHandler(ctx, 1, 1)
	defer teardown()

	ok, err := lease.Effect(ctx, lease.RegisterOf("resource", 1))
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = lease.Effect(ctx, lease.AcquireOf("resource"))
	require.NoError(t, err)
	require.True(t, ok)

	// Try to acquire again — should block, so we use timeout context
	ctxTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	ok, err = lease.Effect(ctxTimeout, lease.AcquireOf("resource"))
	require.Error(t, err)
	require.False(t, ok)

	// Release the lease
	ok, err = lease.Effect(ctx, lease.ReleaseOf("resource"))
	require.NoError(t, err)
	require.True(t, ok)

	// Now it should be acquirable again
	ok, err = lease.Effect(ctx, lease.AcquireOf("resource"))
	require.NoError(t, err)
	require.True(t, ok)

	// Try deregister while lease is held — should fail
	ok, err = lease.Effect(ctx, lease.DeregisterOf("resource"))
	require.Error(t, err)
	require.False(t, ok)

	// Release and then deregister
	ok, err = lease.Effect(ctx, lease.ReleaseOf("resource"))
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = lease.Effect(ctx, lease.DeregisterOf("resource"))
	require.NoError(t, err)
	require.True(t, ok)
}
