package dbsize

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)

const gib = 1 << 30

func TestGrowthLinear(t *testing.T) {
	g, ok := Growth([]Sample{
		{Name: "loja", Bytes: gib, At: t0},
		{Name: "loja", Bytes: gib + gib/4, At: t0.Add(6 * time.Hour)},
		{Name: "loja", Bytes: gib + gib/2, At: t0.Add(12 * time.Hour)},
	})
	require.True(t, ok)
	assert.InDelta(t, 1.0737, g, 1e-3) // 1 GiB por dia, em GB (10⁹ bytes)
}

func TestGrowthShrinkingIsNegative(t *testing.T) {
	g, ok := Growth([]Sample{{Bytes: 2e9, At: t0}, {Bytes: 1e9, At: t0.Add(24 * time.Hour)}})
	require.True(t, ok)
	assert.InDelta(t, -1.0, g, 1e-9)
}

func TestGrowthTooShort(t *testing.T) {
	_, ok := Growth([]Sample{{Bytes: 1, At: t0}, {Bytes: 2, At: t0.Add(30 * time.Minute)}})
	assert.False(t, ok)
	_, ok = Growth([]Sample{{Bytes: 1, At: t0}})
	assert.False(t, ok)
	_, ok = Growth(nil)
	assert.False(t, ok)
}

type fakeSink struct {
	mu     sync.Mutex
	saved  [][]Sample
	pruned int
}

func (s *fakeSink) AddDBSizes(_ context.Context, list []Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saved = append(s.saved, list)
	return nil
}

func (s *fakeSink) PruneDBSizes(context.Context, time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruned++
	return nil
}

func (s *fakeSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.saved)
}

func TestRunKeepsGoingWhenQuerierFails(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	var mu sync.Mutex
	calls := 0
	q := func(context.Context) ([]Sample, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			return nil, errors.New("conexão recusada")
		}
		return []Sample{{Name: "loja", Bytes: 42, At: time.Now()}}, nil
	}
	sink := &fakeSink{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Run(ctx, q, sink, 10*time.Millisecond, time.Hour)
		close(done)
	}()
	require.Eventually(t, func() bool { return sink.count() >= 1 }, 2*time.Second, 5*time.Millisecond)
	cancel()
	<-done
	assert.Contains(t, logs.String(), "conexão recusada")
}

func TestPGQuerierErrorHidesPassword(t *testing.T) {
	q := PGQuerier("postgres://omni:segredo123@127.0.0.1:1/loja?connect_timeout=1")
	_, err := q(context.Background())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "segredo123")
	assert.True(t, strings.HasPrefix(err.Error(), "ler tamanho do PostgreSQL"), err.Error())
}

func TestPGQuerierBadDSNHidesPassword(t *testing.T) {
	q := PGQuerier("postgres://omni:segredo123@127.0.0.1:porta-invalida/loja")
	_, err := q(context.Background())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "segredo123")
}

// TestPGQuerierReadsRealDatabase roda contra um PostgreSQL de verdade quando
// OMNI_TEST_PG_DSN está definido (um papel com pg_monitor basta).
func TestPGQuerierReadsRealDatabase(t *testing.T) {
	dsn := os.Getenv("OMNI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("defina OMNI_TEST_PG_DSN para testar contra um PostgreSQL")
	}
	list, err := PGQuerier(dsn)(context.Background())
	require.NoError(t, err)
	sizes := map[string]int64{}
	for _, s := range list {
		sizes[s.Name] = s.Bytes
	}
	assert.Positive(t, sizes["postgres"], "o banco postgres sempre existe: %v", sizes)
	assert.NotContains(t, sizes, "template0")
}
