package mailqueue

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/ws"
)

func write(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("mensagem"), 0o600))
}

func TestCountIgnoresSymlinksAndDirs(t *testing.T) {
	active, deferred := t.TempDir(), t.TempDir()
	write(t, filepath.Join(active, "A1B2C3"))
	write(t, filepath.Join(deferred, "0", "D4E5F6"))
	write(t, filepath.Join(deferred, "1", "G7H8I9"))
	require.NoError(t, os.MkdirAll(filepath.Join(deferred, "2"), 0o700))
	if runtime.GOOS != "windows" {
		require.NoError(t, os.Symlink(filepath.Join(active, "A1B2C3"), filepath.Join(deferred, "link")))
		require.NoError(t, os.Symlink(active, filepath.Join(deferred, "dirlink")))
	}
	n, err := Count([]string{active, deferred})
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}

func TestCountMissingDirReturnsError(t *testing.T) {
	_, err := Count([]string{t.TempDir(), filepath.Join(t.TempDir(), "nao-existe")})
	require.Error(t, err)
}

type fakePub struct {
	mu     sync.Mutex
	events []ws.Event
}

func (p *fakePub) Publish(ev ws.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, ev)
}

func (p *fakePub) snapshots() []Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []Snapshot
	for _, ev := range p.events {
		if ev.Type == EventQueue {
			out = append(out, ev.Data.(Snapshot))
		}
	}
	return out
}

type fakeSink struct {
	mu    sync.Mutex
	saved []Snapshot
}

func (s *fakeSink) AddQueueSnapshot(_ context.Context, snap Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saved = append(s.saved, snap)
	return nil
}

func (s *fakeSink) PruneQueue(context.Context, time.Time) error { return nil }

func (s *fakeSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.saved)
}

func TestRunPublishesCountAndErrors(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "m1"))
	missing := filepath.Join(t.TempDir(), "nao-existe")

	for name, dirs := range map[string][]string{"ok": {dir}, "erro": {missing}} {
		t.Run(name, func(t *testing.T) {
			pub, sink := &fakePub{}, &fakeSink{}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				Run(ctx, pub, sink, dirs, 10*time.Millisecond, time.Hour)
				close(done)
			}()
			require.Eventually(t, func() bool { return len(pub.snapshots()) >= 2 }, 2*time.Second, 5*time.Millisecond)
			cancel()
			<-done
			snap := pub.snapshots()[0]
			if name == "ok" {
				assert.Equal(t, 1, snap.Messages)
				assert.Empty(t, snap.Err)
				assert.Positive(t, sink.count())
			} else {
				assert.NotEmpty(t, snap.Err)
				assert.Zero(t, sink.count(), "uma contagem que falhou não vai para o histórico")
			}
		})
	}
}
