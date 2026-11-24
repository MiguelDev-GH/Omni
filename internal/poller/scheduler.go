package poller

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/MiguelDev-GH/omni/internal/target"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

// CheckTimeout limita cada verificação HTTP e TCP.
const CheckTimeout = 5 * time.Second

// maxConcurrentChecks é o número máximo de verificações simultâneas numa rodada.
const maxConcurrentChecks = 16

// Tipos de evento publicados pelo agendador.
const (
	EventStatus   = "STATUS_ALVOS"
	EventDown     = "ALERTA_QUEDA"
	EventRestored = "SERVICO_RESTAURADO"
)

// ResultStore é o que o agendador precisa do armazenamento.
type ResultStore interface {
	ListTargets(ctx context.Context) ([]target.Target, error)
	InsertCheckResult(ctx context.Context, r Result) error
	// PruneChecks apaga os resultados anteriores a before.
	PruneChecks(ctx context.Context, before time.Time) error
}

// pruneEvery é o intervalo mínimo entre duas limpezas do histórico.
const pruneEvery = time.Hour

// StatusData é o conteúdo de um evento STATUS_ALVOS.
type StatusData struct {
	Results []Result `json:"resultados"`
}

// DownAlert é o conteúdo de um evento ALERTA_QUEDA.
type DownAlert struct {
	TargetID int64     `json:"alvo_id"`
	Target   string    `json:"alvo"`
	Port     int       `json:"porta"`
	Err      string    `json:"erro"`
	At       time.Time `json:"hora"`
}

// RestoredAlert é o conteúdo de um evento SERVICO_RESTAURADO.
type RestoredAlert struct {
	TargetID int64     `json:"alvo_id"`
	Target   string    `json:"alvo"`
	Port     int       `json:"porta"`
	At       time.Time `json:"hora"`
}

// portKey identifica o estado de uma verificação. Kind separa as verificações
// http e tcp quando porta_banco coincide com uma porta de app.
type portKey struct {
	TargetID int64
	Port     int
	Kind     string
}

type check struct {
	target target.Target
	port   int
	kind   string
}

// Scheduler executa rodadas de verificação, grava os resultados e publica
// o estado dos alvos e as transições de no ar para fora do ar e vice-versa.
type Scheduler struct {
	store    ResultStore
	pub      ws.Publisher
	interval time.Duration
	client   *http.Client
	// last guarda o último estado de cada porta; só a goroutine que chama
	// RunOnce o acessa.
	last map[portKey]bool
	// retention é por quanto tempo os resultados ficam guardados; zero
	// desliga a limpeza.
	retention time.Duration
	lastPrune time.Time
}

func NewScheduler(st ResultStore, pub ws.Publisher, interval time.Duration) *Scheduler {
	return &Scheduler{
		store:    st,
		pub:      pub,
		interval: interval,
		client:   NewHTTPClient(CheckTimeout),
		last:     make(map[portKey]bool),
	}
}

// WithRetention liga a limpeza dos resultados mais antigos que d, feita no
// fim de uma rodada no máximo uma vez por hora.
func (s *Scheduler) WithRetention(d time.Duration) *Scheduler {
	s.retention = d
	return s
}

// Run faz uma rodada imediatamente e depois uma a cada interval, até ctx ser
// cancelado.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		if err := s.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) &&
			!errors.Is(err, context.DeadlineExceeded) {
			slog.Error("rodada do poller", "erro", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RunOnce verifica todas as portas de todos os alvos. Devolve erro se não
// conseguir listar os alvos ou se ctx for encerrado durante a rodada; neste
// caso a rodada é descartada.
func (s *Scheduler) RunOnce(ctx context.Context) error {
	targets, err := s.store.ListTargets(ctx)
	if err != nil {
		return err
	}

	var checks []check
	for _, t := range targets {
		for _, p := range t.AppPorts {
			checks = append(checks, check{target: t, port: p, kind: KindHTTP})
		}
		if t.DBPort != 0 {
			checks = append(checks, check{target: t, port: t.DBPort, kind: KindTCP})
		}
		if t.DNSPort != 0 {
			checks = append(checks, check{target: t, port: t.DNSPort, kind: KindDNS})
		}
		if t.SMTPPort != 0 {
			checks = append(checks, check{target: t, port: t.SMTPPort, kind: KindSMTP})
		}
	}

	results := s.runChecks(ctx, checks)
	// Uma rodada interrompida (desligamento) tem resultados falsos de queda:
	// nada é gravado, publicado ou guardado como estado.
	if err := ctx.Err(); err != nil {
		return err
	}

	seen := make(map[portKey]struct{}, len(checks))
	for i, c := range checks {
		r := results[i]
		if err := s.store.InsertCheckResult(ctx, r); err != nil {
			slog.Error("gravar verificação", "alvo_id", r.TargetID, "porta", r.Port, "erro", err)
		}
		key := portKey{TargetID: c.target.ID, Port: c.port, Kind: r.Kind}
		seen[key] = struct{}{}
		s.transition(key, c.target.Name, r)
	}

	for key := range s.last {
		if _, ok := seen[key]; !ok {
			delete(s.last, key)
		}
	}

	if len(results) > 0 {
		s.pub.Publish(ws.Event{Type: EventStatus, Data: StatusData{Results: results}})
	}
	s.prune(ctx)
	return nil
}

// prune apaga os resultados mais antigos que a retenção, se a última limpeza
// tiver sido há pelo menos pruneEvery. Uma falha é só registrada.
func (s *Scheduler) prune(ctx context.Context) {
	now := time.Now()
	if s.retention <= 0 || now.Sub(s.lastPrune) < pruneEvery {
		return
	}
	s.lastPrune = now
	if err := s.store.PruneChecks(ctx, now.Add(-s.retention)); err != nil && ctx.Err() == nil {
		slog.Error("apagar verificações antigas", "erro", err)
	}
}

// runChecks executa as verificações em paralelo, no máximo
// maxConcurrentChecks por vez, e devolve os resultados na ordem de checks.
func (s *Scheduler) runChecks(ctx context.Context, checks []check) []Result {
	results := make([]Result, len(checks))
	sem := make(chan struct{}, maxConcurrentChecks)
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			switch c.kind {
			case KindTCP:
				results[i] = CheckTCP(ctx, c.target.ID, c.target.Host, c.port, CheckTimeout)
			case KindDNS:
				results[i] = CheckDNS(ctx, c.target.ID, c.target.Host, c.port, c.target.Domain)
			case KindSMTP:
				results[i] = CheckSMTP(ctx, c.target.ID, c.target.Host, c.port, c.target.Domain)
			default:
				results[i] = CheckHTTP(ctx, s.client, c.target.ID, c.target.Host, c.port)
			}
		}()
	}
	wg.Wait()
	return results
}

// transition publica ALERTA_QUEDA ou SERVICO_RESTAURADO quando o estado da
// porta muda. Uma porta vista pela primeira vez fora do ar gera alerta.
func (s *Scheduler) transition(key portKey, name string, r Result) {
	prev, known := s.last[key]
	s.last[key] = r.Up
	switch {
	case !r.Up && (!known || prev):
		s.pub.Publish(ws.Event{Type: EventDown, Data: DownAlert{
			TargetID: key.TargetID, Target: name, Port: key.Port, Err: r.Err, At: r.At,
		}})
	case r.Up && known && !prev:
		s.pub.Publish(ws.Event{Type: EventRestored, Data: RestoredAlert{
			TargetID: key.TargetID, Target: name, Port: key.Port, At: r.At,
		}})
	}
}
