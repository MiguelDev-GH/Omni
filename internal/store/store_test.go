package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpenEnablesWAL(t *testing.T) {
	s := openTest(t)
	var mode string
	require.NoError(t, s.db.QueryRow("PRAGMA journal_mode").Scan(&mode))
	assert.Equal(t, "wal", mode)
}

func TestOpenIsIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "omni.db")

	s, err := Open(ctx, path)
	require.NoError(t, err)
	_, err = s.CreateTarget(ctx, sample())
	require.NoError(t, err)
	require.NoError(t, s.Close())

	s, err = Open(ctx, path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	list, err := s.ListTargets(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	var version int
	require.NoError(t, s.db.QueryRow("PRAGMA user_version").Scan(&version))
	assert.Equal(t, 1, version)
}

func TestOpenFailsOnMissingDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nao-existe", "omni.db")
	_, err := Open(context.Background(), path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
}
