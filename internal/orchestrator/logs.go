package orchestrator

import (
	"bytes"
	"sync"
	"time"

	"github.com/MiguelDev-GH/omni/internal/ws"
)

// logBatcher junta a saída de uma ferramenta em linhas e publica SCAN_LOG em
// lotes a cada intervalo, nunca linha a linha.
type logBatcher struct {
	pub    ws.Publisher
	scanID int64
	tool   string

	mu      sync.Mutex
	partial []byte
	lines   []string
	sent    int
	done    chan struct{}
	stopped chan struct{}
}

func newLogBatcher(pub ws.Publisher, scanID int64, tool string, every time.Duration) *logBatcher {
	l := &logBatcher{pub: pub, scanID: scanID, tool: tool, done: make(chan struct{}), stopped: make(chan struct{})}
	go func() {
		defer close(l.stopped)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				l.flush()
			case <-l.done:
				return
			}
		}
	}()
	return l
}

func (l *logBatcher) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.partial = append(l.partial, p...)
	for {
		i := bytes.IndexByte(l.partial, '\n')
		if i < 0 {
			if len(l.partial) > maxLineLen {
				l.add(string(l.partial[:maxLineLen]))
				l.partial = l.partial[maxLineLen:]
				continue
			}
			return len(p), nil
		}
		l.add(string(bytes.TrimRight(l.partial[:i], "\r")))
		l.partial = l.partial[i+1:]
	}
}

// Line acrescenta uma linha do próprio Omni, depois da linha parcial da
// ferramenta, se houver.
func (l *logBatcher) Line(line string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.partial) > 0 {
		l.add(string(l.partial))
		l.partial = nil
	}
	l.add(line)
}

// add guarda uma linha, cortando linhas longas e parando em maxLogLines.
func (l *logBatcher) add(line string) {
	if len(line) > maxLineLen {
		line = line[:maxLineLen]
	}
	switch {
	case l.sent+len(l.lines) < maxLogLines:
		l.lines = append(l.lines, line)
	case l.sent+len(l.lines) == maxLogLines:
		l.lines = append(l.lines, "(saída truncada)")
	}
}

func (l *logBatcher) flush() {
	l.mu.Lock()
	lines := l.lines
	l.lines = nil
	l.sent += len(lines)
	l.mu.Unlock()
	if len(lines) > 0 {
		l.pub.Publish(ws.Event{Type: EventLog, Data: map[string]any{"scan_id": l.scanID, "ferramenta": l.tool, "linhas": lines}})
	}
}

// Close publica o que falta, inclusive a última linha sem quebra, e para o lote.
func (l *logBatcher) Close() {
	close(l.done)
	<-l.stopped
	l.mu.Lock()
	if len(l.partial) > 0 {
		l.add(string(l.partial))
		l.partial = nil
	}
	l.mu.Unlock()
	l.flush()
}
