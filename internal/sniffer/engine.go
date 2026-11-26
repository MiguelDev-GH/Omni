package sniffer

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"

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
	// EventDB leva a l7.DBWindow dos últimos 10 s.
	EventDB = "METRICAS_BANCO"
	// EventDNS leva a l7.DNSWindow dos últimos 10 s.
	EventDNS = "METRICAS_DNS"
	// EventSMTP leva a l7.SMTPWindow dos últimos 10 s.
	EventSMTP = "METRICAS_SMTP"

	StateActive       = "ativo"
	StateNoPermission = "sem-permissao"
	StateInactive     = "inativo"

	defaultWindow      = time.Second
	defaultStatusEvery = 5 * time.Second
	defaultL7Window    = 10 * time.Second
	// Retention é por quanto tempo as janelas ficam no banco, salvo WithRetention.
	Retention  = 24 * time.Hour
	pruneEvery = time.Minute
)

// TrafficStore é o que o motor usa do banco.
type TrafficStore interface {
	ListTargets(ctx context.Context) ([]target.Target, error)
	InsertTrafficWindow(ctx context.Context, w Window) error
	InsertHTTPWindow(ctx context.Context, w l7.HTTPWindow) error
	InsertDBWindow(ctx context.Context, w l7.DBWindow) error
	InsertDNSWindow(ctx context.Context, w l7.DNSWindow) error
	InsertSMTPWindow(ctx context.Context, w l7.SMTPWindow) error
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
	retention   time.Duration

	mu        sync.Mutex
	state     string
	lastPrune time.Time
	onHTTP    func(l7.HTTPExchange)

	tap atomic.Pointer[func(PacketSummary)]

	// rawTaps é trocado inteiro a cada mudança (cópia na escrita), para a
	// goroutine da captura ler sem trava.
	rawMu   sync.Mutex
	rawNext int
	rawTaps atomic.Pointer[[]rawTap]
}

// RawTap recebe cada pacote capturado, como veio da fonte, e o tipo de
// enlace da fonte. Roda na goroutine da captura e não pode bloquear.
type RawTap func(p gopacket.Packet, link layers.LinkType)

type rawTap struct {
	id int
	fn RawTap
}

// linkTyper é a fonte que sabe o próprio tipo de enlace; sem ele, vale Ethernet.
type linkTyper interface {
	LinkType() layers.LinkType
}

// OnHTTP registra uma função chamada a cada requisição HTTP decodificada,
// na goroutine da captura. Não deve bloquear. Chame antes de Run.
func (e *Engine) OnHTTP(fn func(l7.HTTPExchange)) { e.onHTTP = fn }

func NewEngine(open Opener, st TrafficStore, pub ws.Publisher) *Engine {
	return &Engine{
		open: open, st: st, pub: pub,
		window: defaultWindow, statusEvery: defaultStatusEvery, l7Window: defaultL7Window, retention: Retention,
		state: StateInactive,
	}
}

// WithRetention troca por quanto tempo as janelas ficam no banco.
func (e *Engine) WithRetention(d time.Duration) *Engine {
	e.retention = d
	return e
}

// SetTap passa a chamar tap com o resumo de cada pacote capturado, na
// goroutine da captura; tap não pode bloquear. nil desliga o tap.
func (e *Engine) SetTap(tap func(PacketSummary)) {
	if tap == nil {
		e.tap.Store(nil)
		return
	}
	e.tap.Store(&tap)
}

// AddRawTap passa a chamar fn com cada pacote capturado, até a função
// devolvida ser chamada. Vários taps podem ficar ligados ao mesmo tempo.
func (e *Engine) AddRawTap(fn RawTap) (remove func()) {
	e.rawMu.Lock()
	defer e.rawMu.Unlock()
	e.rawNext++
	id := e.rawNext
	e.storeRawTaps(append(e.loadRawTaps(), rawTap{id: id, fn: fn}))
	var once sync.Once
	return func() {
		once.Do(func() {
			e.rawMu.Lock()
			defer e.rawMu.Unlock()
			var kept []rawTap
			for _, t := range e.loadRawTaps() {
				if t.id != id {
					kept = append(kept, t)
				}
			}
			e.storeRawTaps(kept)
		})
	}
}

// loadRawTaps devolve uma cópia da lista, que pode ser alterada à vontade.
func (e *Engine) loadRawTaps() []rawTap {
	if p := e.rawTaps.Load(); p != nil {
		return append([]rawTap(nil), *p...)
	}
	return nil
}

func (e *Engine) storeRawTaps(ts []rawTap) {
	if len(ts) == 0 {
		e.rawTaps.Store(nil)
		return
	}
	e.rawTaps.Store(&ts)
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
			e.captureFrom(ctx, bpf, RolesFor(ts), DBPorts(ts))
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
func (e *Engine) captureFrom(ctx context.Context, bpf string, roles PortRoles, db []int) {
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
	link := layers.LinkTypeEthernet
	if lt, ok := src.(linkTyper); ok {
		link = lt.LinkType()
	}

	capCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	pkts := src.Packets(capCtx)
	agg := NewAggregator(roles)
	agg.Flush(time.Now())
	httpStats := l7.NewHTTPStats()
	httpStats.Flush(time.Now())
	httpAsm := l7.NewHTTPAssembler(roles.App, func(ex l7.HTTPExchange) {
		httpStats.Add(ex)
		if e.onHTTP != nil {
			e.onHTTP(ex)
		}
	})
	dbStats := l7.NewDBStats()
	dbStats.Flush(time.Now())
	pgAsm := l7.NewPostgresAssembler(db, dbStats.Add)
	pgAsm.OnTxFailed(func(time.Time) { dbStats.AddTxFailure() })
	dnsStats := l7.NewDNSStats()
	dnsStats.Flush(time.Now())
	dnsTracker := l7.NewDNSTracker(roles.DNS, dnsStats.Add)
	smtpStats := l7.NewSMTPStats()
	smtpStats.Flush(time.Now())
	smtpAsm := l7.NewSMTPAssembler(roles.SMTP, smtpStats.Add)
	var dropped uint64
	flush := func(now time.Time) {
		w := agg.Flush(now)
		total := src.Stats().Dropped
		w.Dropped, dropped = total-dropped, total
		e.emit(ctx, w)
	}
	flushL7 := func(now time.Time) {
		httpAsm.Flush(now)
		pgAsm.Flush(now)
		dnsTracker.Flush(now)
		smtpAsm.Flush(now)
		e.emitHTTP(ctx, httpStats.Flush(now))
		e.emitDB(ctx, dbStats.Flush(now))
		e.emitDNS(ctx, dnsStats.Flush(now))
		e.emitSMTP(ctx, smtpStats.Flush(now))
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
			pgAsm.Add(p)
			dnsTracker.Add(p)
			smtpAsm.Add(p)
			if tap := e.tap.Load(); tap != nil {
				(*tap)(Summarize(p, roles, db))
			}
			if raw := e.rawTaps.Load(); raw != nil {
				for _, t := range *raw {
					t.fn(p, link)
				}
			}
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

func (e *Engine) emitDB(ctx context.Context, w l7.DBWindow) {
	if w.Queries > 0 {
		if err := e.st.InsertDBWindow(ctx, w); err != nil && ctx.Err() == nil {
			slog.Error("sniffer: gravar janela do banco", "erro", err)
		}
	}
	e.pub.Publish(ws.Event{Type: EventDB, Data: w})
}

func (e *Engine) emitDNS(ctx context.Context, w l7.DNSWindow) {
	if w.Queries > 0 {
		if err := e.st.InsertDNSWindow(ctx, w); err != nil && ctx.Err() == nil {
			slog.Error("sniffer: gravar janela dns", "erro", err)
		}
	}
	e.pub.Publish(ws.Event{Type: EventDNS, Data: w})
}

func (e *Engine) emitSMTP(ctx context.Context, w l7.SMTPWindow) {
	if w.Attempts > 0 {
		if err := e.st.InsertSMTPWindow(ctx, w); err != nil && ctx.Err() == nil {
			slog.Error("sniffer: gravar janela smtp", "erro", err)
		}
	}
	e.pub.Publish(ws.Event{Type: EventSMTP, Data: w})
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

// prune apaga as janelas mais antigas que a retenção, no máximo uma vez por minuto.
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
	if err := e.st.PruneTraffic(ctx, now.Add(-e.retention)); err != nil && ctx.Err() == nil {
		slog.Error("sniffer: apagar janelas antigas", "erro", err)
	}
}

// DBPorts devolve as portas de banco decodificadas como PostgreSQL; a porta
// padrão do MySQL fica de fora.
func DBPorts(ts []target.Target) []int {
	var ports []int
	for _, t := range ts {
		if t.DBPort > 0 && t.DBPort != 3306 {
			ports = append(ports, t.DBPort)
		}
	}
	return ports
}
