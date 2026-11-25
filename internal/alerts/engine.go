package alerts

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// historyLimit é quantos alertas ficam guardados em memória.
const historyLimit = 200

// staleGap é o intervalo sem amostras depois do qual o tempo acumulado de
// uma condição é zerado: uma amostra ruim depois de horas de silêncio não
// conta como "ruim há horas".
const staleGap = 2 * time.Minute

// Sample é uma medição de uma regra para um recurso. Subject identifica o
// recurso (por exemplo "loja:3000"); vazio vale para o servidor inteiro.
// Service, quando preenchido, substitui o serviço da regra.
type Sample struct {
	Rule    string
	Service string
	Subject string
	Value   float64
	Detail  string
	At      time.Time
}

// Alert é uma mudança de nível de um recurso.
type Alert struct {
	ID        int64     `json:"id"`
	Rule      string    `json:"regra"`
	Service   string    `json:"servico"`
	Name      string    `json:"nome"`
	Subject   string    `json:"recurso"`
	Level     Level     `json:"nivel"`
	Previous  Level     `json:"nivel_anterior"`
	Value     float64   `json:"valor"`
	Threshold float64   `json:"limite"`
	Unit      string    `json:"unidade"`
	Detail    string    `json:"detalhe,omitempty"`
	At        time.Time `json:"hora"`
}

// Title resume o alerta numa linha, para notificações.
func (a Alert) Title() string {
	subject := a.Subject
	if subject == "" {
		subject = a.Service
	}
	switch a.Level {
	case Green:
		return fmt.Sprintf("%s normalizado: %s", a.Name, subject)
	case Red:
		return fmt.Sprintf("CRÍTICO: %s em %s", a.Name, subject)
	default:
		return fmt.Sprintf("Atenção: %s em %s", a.Name, subject)
	}
}

// Status é o nível atual de uma regra para um recurso.
type Status struct {
	Rule      string    `json:"regra"`
	Service   string    `json:"servico"`
	Name      string    `json:"nome"`
	Subject   string    `json:"recurso"`
	Level     Level     `json:"nivel"`
	Value     float64   `json:"valor"`
	Unit      string    `json:"unidade"`
	Warn      float64   `json:"limite_amarelo"`
	Crit      float64   `json:"limite_vermelho"`
	Since     time.Time `json:"desde"`
	UpdatedAt time.Time `json:"atualizado"`
}

type stateKey struct {
	rule    string
	subject string
}

// state guarda, para um recurso, o nível confirmado e desde quando cada
// faixa de valores se mantém sem interrupção.
type state struct {
	service   string
	level     Level
	since     time.Time
	value     float64
	lastAt    time.Time
	okSince   time.Time // valor abaixo de Warn
	warnSince time.Time // valor maior ou igual a Warn (inclui os vermelhos)
	critSince time.Time // valor maior ou igual a Crit
}

// Listener recebe cada alerta. É chamado fora da trava do Engine e não deve
// bloquear.
type Listener func(Alert)

// Engine aplica as regras às amostras e guarda o estado e o histórico.
// É seguro para uso concorrente.
type Engine struct {
	mu        sync.Mutex
	rules     map[string]Rule
	states    map[stateKey]*state
	history   []Alert // do mais antigo para o mais novo
	nextID    int64
	listeners []Listener
}

// NewEngine cria um Engine com as regras dadas.
func NewEngine(rules []Rule) *Engine {
	m := make(map[string]Rule, len(rules))
	for _, r := range rules {
		m[r.ID] = r
	}
	return &Engine{rules: m, states: make(map[stateKey]*state), nextID: 1}
}

// OnAlert registra um Listener. Registre todos antes da primeira amostra.
func (e *Engine) OnAlert(l Listener) {
	e.mu.Lock()
	e.listeners = append(e.listeners, l)
	e.mu.Unlock()
}

// Rules devolve as regras em ordem de serviço e identificador.
func (e *Engine) Rules() []Rule {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Rule, 0, len(e.rules))
	for _, r := range e.rules {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Service != out[j].Service {
			return out[i].Service < out[j].Service
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Observe aplica uma amostra. Amostras de regras desconhecidas são ignoradas.
func (e *Engine) Observe(s Sample) {
	e.mu.Lock()
	alert, ok := e.observe(s)
	listeners := e.listeners
	e.mu.Unlock()
	if !ok {
		return
	}
	for _, l := range listeners {
		l(alert)
	}
}

func (e *Engine) observe(s Sample) (Alert, bool) {
	rule, ok := e.rules[s.Rule]
	if !ok {
		slog.Warn("alertas: regra desconhecida", "regra", s.Rule)
		return Alert{}, false
	}
	service := s.Service
	if service == "" {
		service = rule.Service
	}
	key := stateKey{rule: s.Rule, subject: s.Subject}
	st, known := e.states[key]
	if !known {
		st = &state{service: service, level: Green, since: s.At}
		e.states[key] = st
	}
	if !st.lastAt.IsZero() && s.At.Sub(st.lastAt) > staleGap {
		st.okSince, st.warnSince, st.critSince = time.Time{}, time.Time{}, time.Time{}
	}
	st.lastAt = s.At
	st.value = s.Value
	st.service = service

	// Atualiza desde quando cada faixa se mantém.
	switch rule.LevelFor(s.Value) {
	case Red:
		st.okSince = time.Time{}
		st.warnSince = keep(st.warnSince, s.At)
		st.critSince = keep(st.critSince, s.At)
	case Yellow:
		st.okSince = time.Time{}
		st.warnSince = keep(st.warnSince, s.At)
		st.critSince = time.Time{}
	default:
		st.okSince = keep(st.okSince, s.At)
		st.warnSince, st.critSince = time.Time{}, time.Time{}
	}

	next := st.level
	held := func(since time.Time) bool { return !since.IsZero() && s.At.Sub(since) >= rule.For }
	switch {
	case held(st.critSince):
		next = Red
	case held(st.warnSince):
		next = Yellow
	case held(st.okSince):
		next = Green
	}
	if next == st.level {
		return Alert{}, false
	}

	prev := st.level
	st.level = next
	st.since = s.At
	a := Alert{
		ID:        e.nextID,
		Rule:      rule.ID,
		Service:   service,
		Name:      rule.Name,
		Subject:   s.Subject,
		Level:     next,
		Previous:  prev,
		Value:     s.Value,
		Threshold: rule.threshold(next),
		Unit:      rule.Unit,
		Detail:    s.Detail,
		At:        s.At,
	}
	if next == Green {
		a.Threshold = rule.Warn
	}
	e.nextID++
	e.history = append(e.history, a)
	if len(e.history) > historyLimit {
		e.history = e.history[len(e.history)-historyLimit:]
	}
	return a, true
}

func keep(since, now time.Time) time.Time {
	if since.IsZero() {
		return now
	}
	return since
}

// Forget apaga o estado dos recursos para os quais drop(serviço, recurso)
// é verdadeiro, por exemplo as portas de um alvo removido ou um IP que
// parou de mandar tráfego.
func (e *Engine) Forget(drop func(service, subject string) bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for k, st := range e.states {
		if k.subject != "" && drop(st.service, k.subject) {
			delete(e.states, k)
		}
	}
}

// Drop apaga o estado de uma regra para um recurso, por exemplo um IP que
// parou de atacar e já voltou ao verde.
func (e *Engine) Drop(rule, subject string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.states, stateKey{rule: rule, subject: subject})
}

// Snapshot devolve o nível atual de cada recurso, do mais grave para o mais
// leve, e o histórico do mais novo para o mais antigo.
func (e *Engine) Snapshot() ([]Status, []Alert) {
	e.mu.Lock()
	defer e.mu.Unlock()
	statuses := make([]Status, 0, len(e.states))
	for k, st := range e.states {
		r := e.rules[k.rule]
		statuses = append(statuses, Status{
			Rule: r.ID, Service: st.service, Name: r.Name, Subject: k.subject,
			Level: st.level, Value: st.value, Unit: r.Unit, Warn: r.Warn, Crit: r.Crit,
			Since: st.since, UpdatedAt: st.lastAt,
		})
	}
	sort.Slice(statuses, func(i, j int) bool {
		a, b := statuses[i], statuses[j]
		if a.Level.rank() != b.Level.rank() {
			return a.Level.rank() > b.Level.rank()
		}
		if a.Service != b.Service {
			return a.Service < b.Service
		}
		if a.Subject != b.Subject {
			return a.Subject < b.Subject
		}
		return a.Rule < b.Rule
	})
	history := make([]Alert, len(e.history))
	for i, a := range e.history {
		history[len(e.history)-1-i] = a
	}
	return statuses, history
}
