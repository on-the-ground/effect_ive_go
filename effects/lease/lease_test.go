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

	ctx, endOfLeaseHandler := lease.WithInMemoryEffectHandler(ctx, 1, 1)
	defer endOfLeaseHandler()

	ok, err := lease.ResourceRegistrationNoExpiryEffect(ctx, "resource", 1)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = lease.AcquisitionEffect(ctx, "resource")
	require.NoError(t, err)
	require.True(t, ok)

	// Try to acquire again — should block, so we use timeout context
	ctxTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	ok, err = lease.AcquisitionEffect(ctxTimeout, "resource")
	require.Error(t, err)
	require.False(t, ok)

	// Release the lease
	ok, err = lease.ReleaseEffect(ctx, "resource")
	require.NoError(t, err)
	require.True(t, ok)

	// Now it should be acquirable again
	ok, err = lease.AcquisitionEffect(ctx, "resource")
	require.NoError(t, err)
	require.True(t, ok)

	// Try deregister while lease is held — should fail
	ok, err = lease.ResourceDeregistrationEffect(ctx, "resource")
	require.Error(t, err)
	require.False(t, ok)

	// Release and then deregister
	ok, err = lease.ReleaseEffect(ctx, "resource")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = lease.ResourceDeregistrationEffect(ctx, "resource")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestLease_TTL_AcquireAndRelease(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, endOfLeaseHandler := lease.WithInMemoryEffectHandler(ctx, 10, 2)
	defer endOfLeaseHandler()

	key := "resource/ttl"
	ttl := 100 * time.Millisecond
	pollInterval := 10 * time.Millisecond

	// 등록
	ok, err := lease.ResourceRegistrationEffect(ctx, key, 1, ttl, pollInterval)
	require.NoError(t, err)
	require.True(t, ok, "lease registration should succeed")

	// acquire
	ok, err = lease.AcquisitionEffect(ctx, key)
	require.NoError(t, err)
	require.True(t, ok, "lease acquisition should succeed")

	// wait past ttl
	time.Sleep(1 * time.Second)

	// release (sink에는 아무 것도 없을 것)
	ok, err = lease.ReleaseEffect(ctx, key)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = lease.ResourceDeregistrationEffect(ctx, key)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestLease_TTL_AcquireAndTimelyRelease(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, endOfLeaseHandler := lease.WithInMemoryEffectHandler(ctx, 10, 2)
	defer endOfLeaseHandler()

	key := "resource/quick"
	ttl := 500 * time.Millisecond
	pollInterval := 10 * time.Millisecond

	ok, err := lease.ResourceRegistrationEffect(ctx, key, 1, ttl, pollInterval)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = lease.AcquisitionEffect(ctx, key)
	require.NoError(t, err)
	require.True(t, ok)

	// wait less than TTL
	time.Sleep(100 * time.Millisecond)

	ok, err = lease.ReleaseEffect(ctx, key)
	require.NoError(t, err)
	require.True(t, ok, "release should succeed before TTL expires")

	ok, err = lease.ResourceDeregistrationEffect(ctx, key)
	require.NoError(t, err)
	require.True(t, ok)
}
