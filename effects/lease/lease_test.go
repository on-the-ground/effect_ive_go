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

	ok, err := lease.EffectResourceRegistration(ctx, "resource", 1)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = lease.EffectAcquisition(ctx, "resource")
	require.NoError(t, err)
	require.True(t, ok)

	// Try to acquire again — should block, so we use timeout context
	ctxTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	ok, err = lease.EffectAcquisition(ctxTimeout, "resource")
	require.Error(t, err)
	require.False(t, ok)

	// Release the lease
	ok, err = lease.EffectRelease(ctx, "resource")
	require.NoError(t, err)
	require.True(t, ok)

	// Now it should be acquirable again
	ok, err = lease.EffectAcquisition(ctx, "resource")
	require.NoError(t, err)
	require.True(t, ok)

	// Try deregister while lease is held — should fail
	ok, err = lease.EffectResourceDeregistration(ctx, "resource")
	require.Error(t, err)
	require.False(t, ok)

	// Release and then deregister
	ok, err = lease.EffectRelease(ctx, "resource")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = lease.EffectResourceDeregistration(ctx, "resource")
	require.NoError(t, err)
	require.True(t, ok)
}
