package sysmetrics

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/ws"
)

type fakePub struct {
	mu     sync.Mutex
	events []ws.Event
}

func (p *fakePub) Publish(ev ws.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, ev)
}

func (p *fakePub) all() []ws.Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]ws.Event(nil), p.events...)
}

func TestSampleInRange(t *testing.T) {
	snap, err := Sample(context.Background(), t.TempDir())
	require.NoError(t, err)
	for _, v := range []float64{snap.CPUPercent, snap.MemPercent, snap.DiskPercent} {
		assert.GreaterOrEqual(t, v, 0.0)
		assert.LessOrEqual(t, v, 100.0)
	}
	assert.False(t, snap.At.IsZero())
}

func TestSampleBadDiskPath(t *testing.T) {
	_, err := Sample(context.Background(), "/caminho/que/nao/existe")
	require.Error(t, err)
}

func TestRunPublishesUntilCancel(t *testing.T) {
	pub := &fakePub{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		Run(ctx, pub, t.TempDir(), 20*time.Millisecond)
		close(done)
	}()

	require.Eventually(t, func() bool { return len(pub.all()) >= 2 }, 2*time.Second, 10*time.Millisecond)
	for _, ev := range pub.all() {
		assert.Equal(t, "METRICAS_SISTEMA", ev.Type)
		_, ok := ev.Data.(Snapshot)
		assert.True(t, ok)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run não retornou após o cancelamento")
	}
}
