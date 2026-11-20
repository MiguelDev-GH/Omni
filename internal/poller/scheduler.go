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
}

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

type portKey struct {
	TargetID int64
	Port     int
}

type check struct {
	target target.Target
	port   int
	tcp    bool
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
			checks = append(checks, check{target: t, port: p})
		}
		if t.DBPort != 0 {
			checks = append(checks, check{target: t, port: t.DBPort, tcp: true})
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
		key := portKey{TargetID: c.target.ID, Port: c.port}
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
	return nil
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
			if c.tcp {
				results[i] = CheckTCP(ctx, c.target.ID, c.target.Host, c.port, CheckTimeout)
			} else {
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
