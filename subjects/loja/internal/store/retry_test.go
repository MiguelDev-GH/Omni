package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeMigrator struct {
	calls, failures int
}

func (f *fakeMigrator) Migrate(context.Context) error {
	f.calls++
	if f.calls <= f.failures {
		return ErrUnavailable
	}
	return nil
}

func TestMigrateRetriesUntilDB(t *testing.T) {
	f := &fakeMigrator{failures: 2}
	assert.NoError(t, MigrateWithRetry(context.Background(), f, time.Millisecond))
	assert.Equal(t, 3, f.calls)
}

func TestMigrateRetryStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &fakeMigrator{failures: 1 << 30}
	assert.ErrorIs(t, MigrateWithRetry(ctx, f, time.Millisecond), context.Canceled)
}
