package store

import (
	"context"
	"testing"

	"github.com/MiguelDev-GH/omni/internal/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sample() target.Target {
	return target.Target{Name: "api", Host: "127.0.0.1", AppPorts: []int{3000, 3001}, DBPort: 5432, Domain: "app.com"}
}

func TestListTargetsEmpty(t *testing.T) {
	list, err := openTest(t).ListTargets(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Len(t, list, 0)
}

func TestTargetCRUDRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)

	created, err := s.CreateTarget(ctx, sample())
	require.NoError(t, err)
	assert.Positive(t, created.ID)

	got, err := s.GetTarget(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, got)

	created.Name = "renomeado"
	require.NoError(t, s.UpdateTarget(ctx, created))
	got, err = s.GetTarget(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "renomeado", got.Name)

	require.NoError(t, s.DeleteTarget(ctx, created.ID))
	_, err = s.GetTarget(ctx, created.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListTargetsOrderedByID(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	for range 3 {
		_, err := s.CreateTarget(ctx, sample())
		require.NoError(t, err)
	}
	list, err := s.ListTargets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Less(t, list[0].ID, list[1].ID)
	assert.Less(t, list[1].ID, list[2].ID)
}

func TestMissingTargetErrors(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)

	_, err := s.GetTarget(ctx, 999)
	assert.ErrorIs(t, err, ErrNotFound)

	missing := sample()
	missing.ID = 999
	assert.ErrorIs(t, s.UpdateTarget(ctx, missing), ErrNotFound)

	assert.ErrorIs(t, s.DeleteTarget(ctx, 999), ErrNotFound)
}

func TestTargetDNSAndSMTPPortsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)

	in := sample()
	in.DNSPort, in.SMTPPort = 1053, 1025
	created, err := s.CreateTarget(ctx, in)
	require.NoError(t, err)
	got, err := s.GetTarget(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, 1053, got.DNSPort)
	assert.Equal(t, 1025, got.SMTPPort)

	got.DNSPort, got.SMTPPort = 0, 2525
	require.NoError(t, s.UpdateTarget(ctx, got))
	list, err := s.ListTargets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, 0, list[0].DNSPort)
	assert.Equal(t, 2525, list[0].SMTPPort)
}

func TestTargetRepoRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)

	in := sample()
	in.Repo = "/srv/loja"
	created, err := s.CreateTarget(ctx, in)
	require.NoError(t, err)
	got, err := s.GetTarget(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "/srv/loja", got.Repo)

	got.Repo = ""
	require.NoError(t, s.UpdateTarget(ctx, got))
	list, err := s.ListTargets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Empty(t, list[0].Repo)
}
