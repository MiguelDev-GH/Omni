package sniffer

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
	"github.com/MiguelDev-GH/omni/internal/target"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

const (
	// EventTraffic leva a Window do último segundo.
	EventTraffic = "METRICAS_TRAFEGO"
	// EventEngines leva o estado do sniffer, como {"sniffer": "ativo"}.
	EventEngines = "ESTADO_MOTORES"
	// EventHTTP leva a l7.HTTPWindow dos últimos 10 s.
	EventHTTP = "METRICAS_HTTP"

	StateActive       = "ativo"
	StateNoPermission = "sem-permissao"
	StateInactive     = "inativo"

	defaultWindow      = time.Second
	defaultStatusEvery = 5 * time.Second
	defaultL7Window    = 10 * time.Second
	// Retention é por quanto tempo as janelas ficam no banco.
	Retention  = 24 * time.Hour
	pruneEvery = time.Minute
)

// TrafficStore é o que o motor usa do banco.
type TrafficStore interface {
	ListTargets(ctx context.Context) ([]target.Target, error)
	InsertTrafficWindow(ctx context.Context, w Window) error
	InsertHTTPWindow(ctx context.Context, w l7.HTTPWindow) error
	// PruneTraffic apaga as janelas de todas as métricas do sniffer anteriores a before.
	PruneTraffic(ctx context.Context, before time.Time) error
}

// Opener abre uma fonte de pacotes com o filtro BPF dado.
type Opener func(bpf string) (Source, error)

// Engine captura o tráfego das portas dos alvos, grava uma janela por segundo
// e publica as métricas e o próprio estado. Sem permissão de captura, só
// publica o estado e tenta de novo a cada ciclo, sem derrubar o resto do Omni.
type Engine struct {
	open        Opener
	st          TrafficStore
	pub         ws.Publisher
	window      time.Duration
	statusEvery time.Duration
	l7Window    time.Duration

	mu        sync.Mutex
	state     string
	lastPrune time.Time
}

func NewEngine(open Opener, st TrafficStore, pub ws.Publisher) *Engine {
	return &Engine{
		open: open, st: st, pub: pub,
		window: defaultWindow, statusEvery: defaultStatusEvery, l7Window: defaultL7Window,
		state: StateInactive,
	}
}

// State devolve "ativo", "sem-permissao" ou "inativo".
func (e *Engine) State() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

// Run captura até ctx ser cancelado. Sem alvos, ou quando a fonte falha ou
// acaba, espera um ciclo de estado e tenta de novo.
func (e *Engine) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		e.prune(ctx)
		ts, err := e.st.ListTargets(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("sniffer: listar alvos", "erro", err)
		}
		if bpf := BPFForTargets(ts); bpf != "" {
			e.captureFrom(ctx, bpf, appPorts(ts))
		} else {
			e.setState(StateInactive, nil)
		}
		select {
		case <-ctx.Done():
		case <-time.After(e.statusEvery):
		}
	}
	return nil
}

// captureFrom abre a fonte com bpf e agrega até ctx acabar, a fonte fechar ou
// o filtro dos alvos mudar.
func (e *Engine) captureFrom(ctx context.Context, bpf string, app []int) {
	src, err := e.open(bpf)
	if err != nil {
		if errors.Is(err, ErrSemPermissao) {
			e.setState(StateNoPermission, err)
		} else {
			e.setState(StateInactive, err)
		}
		return
	}
	defer func() { _ = src.Close() }()
	e.setState(StateActive, nil)

	capCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	pkts := src.Packets(capCtx)
	agg := NewAggregator(app)
	agg.Flush(time.Now())
	httpStats := l7.NewHTTPStats()
	httpStats.Flush(time.Now())
	httpAsm := l7.NewHTTPAssembler(app, httpStats.Add)
	var dropped uint64
	flush := func(now time.Time) {
		w := agg.Flush(now)
		total := src.Stats().Dropped
		w.Dropped, dropped = total-dropped, total
		e.emit(ctx, w)
	}
	flushL7 := func(now time.Time) {
		httpAsm.Flush(now)
		e.emitHTTP(ctx, httpStats.Flush(now))
	}

	winT := time.NewTicker(e.window)
	defer winT.Stop()
	statT := time.NewTicker(e.statusEvery)
	defer statT.Stop()
	l7T := time.NewTicker(e.l7Window)
	defer l7T.Stop()
	for {
		select {
		case p, ok := <-pkts:
			if !ok {
				if ctx.Err() == nil {
					flush(time.Now())
					flushL7(time.Now())
					e.setState(StateInactive, errors.New("a fonte de pacotes terminou"))
				}
				return
			}
			agg.Add(p)
			httpAsm.Add(p)
		case now := <-winT.C:
			flush(now)
		case now := <-l7T.C:
			flushL7(now)
		case <-statT.C:
			e.publishState()
			e.prune(ctx)
			ts, err := e.st.ListTargets(ctx)
			if err == nil && BPFForTargets(ts) != bpf {
				flush(time.Now())
				flushL7(time.Now())
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (e *Engine) emit(ctx context.Context, w Window) {
	if w.Packets > 0 {
		if err := e.st.InsertTrafficWindow(ctx, w); err != nil && ctx.Err() == nil {
			slog.Error("sniffer: gravar janela", "erro", err)
		}
	}
	e.pub.Publish(ws.Event{Type: EventTraffic, Data: w})
}

func (e *Engine) emitHTTP(ctx context.Context, w l7.HTTPWindow) {
	if w.Requests > 0 {
		if err := e.st.InsertHTTPWindow(ctx, w); err != nil && ctx.Err() == nil {
			slog.Error("sniffer: gravar janela http", "erro", err)
		}
	}
	e.pub.Publish(ws.Event{Type: EventHTTP, Data: w})
}

// setState grava o estado, registra a mudança no log e publica o estado.
func (e *Engine) setState(s string, cause error) {
	e.mu.Lock()
	changed := e.state != s
	e.state = s
	e.mu.Unlock()
	if changed {
		if cause != nil {
			slog.Warn("sniffer "+s, "motivo", cause)
		} else {
			slog.Info("sniffer " + s)
		}
	}
	e.publishState()
}

func (e *Engine) publishState() {
	e.pub.Publish(ws.Event{Type: EventEngines, Data: map[string]string{"sniffer": e.State()}})
}

// prune apaga as janelas mais antigas que Retention, no máximo uma vez por minuto.
func (e *Engine) prune(ctx context.Context) {
	now := time.Now()
	e.mu.Lock()
	due := now.Sub(e.lastPrune) >= pruneEvery
	if due {
		e.lastPrune = now
	}
	e.mu.Unlock()
	if !due {
		return
	}
	if err := e.st.PruneTraffic(ctx, now.Add(-Retention)); err != nil && ctx.Err() == nil {
		slog.Error("sniffer: apagar janelas antigas", "erro", err)
	}
}

// appPorts devolve só as portas de aplicação, que o classificador trata como HTTP.
func appPorts(ts []target.Target) []int {
	var ports []int
	for _, t := range ts {
		ports = append(ports, t.AppPorts...)
	}
	return ports
}
