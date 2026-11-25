package l7

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// HTTPWindow resume as requisições HTTP de um intervalo, normalmente 10 s.
type HTTPWindow struct {
	Start    time.Time         `json:"inicio"`
	Requests uint64            `json:"requisicoes"`
	ByStatus map[string]uint64 `json:"status"` // "2xx", "3xx", "4xx", "5xx"
	// ByCode conta as respostas de erro (status >= 400) por código, como "404".
	ByCode map[string]uint64 `json:"codigos"`
	P50MS  float64           `json:"p50_ms"`
	P95MS  float64           `json:"p95_ms"`
	Routes []RouteStats      `json:"rotas"`
}

// RouteStats resume uma rota numa janela. Errors conta as respostas 5xx.
type RouteStats struct {
	Method   string    `json:"metodo"`
	Route    string    `json:"rota"`
	Requests uint64    `json:"requisicoes"`
	Errors   uint64    `json:"erros"`
	P50MS    float64   `json:"p50_ms"`
	P95MS    float64   `json:"p95_ms"`
	Hist     Histogram `json:"-"`
}

// HTTPStats agrega os HTTPExchange da janela atual. Não é seguro para uso
// concorrente.
type HTTPStats struct {
	start    time.Time
	all      group
	byStatus map[string]uint64
	byCode   map[string]uint64
	routes   map[string]*group
}

func NewHTTPStats() *HTTPStats {
	s := &HTTPStats{}
	s.reset(time.Time{})
	return s
}

func (s *HTTPStats) reset(start time.Time) {
	s.start = start
	s.all = group{}
	s.byStatus = map[string]uint64{}
	s.byCode = map[string]uint64{}
	s.routes = map[string]*group{}
}

// Add conta ex na janela; o tráfego do próprio Omni (Internal) fica de fora.
func (s *HTTPStats) Add(ex HTTPExchange) {
	if ex.Internal {
		return
	}
	isErr := ex.Status >= 500
	s.all.observe(ex.Latency, isErr)
	s.byStatus[StatusClass(ex.Status)]++
	if ex.Status >= 400 {
		s.byCode[strconv.Itoa(ex.Status)]++
	}
	key := ex.Method + " " + ex.Route
	g := s.routes[key]
	if g == nil {
		g = &group{}
		s.routes[key] = g
	}
	g.observe(ex.Latency, isErr)
}

// Flush devolve a janela iniciada no Flush anterior e começa outra em now.
// A primeira janela começa em now.
func (s *HTTPStats) Flush(now time.Time) HTTPWindow {
	start := s.start
	if start.IsZero() {
		start = now
	}
	w := HTTPWindow{Start: start.UTC(), Requests: s.all.count, ByStatus: s.byStatus, ByCode: s.byCode, Routes: []RouteStats{}}
	w.P50MS, w.P95MS = s.all.quantiles()
	keys, rest := topGroups(s.routes)
	for _, k := range keys {
		method, route, _ := strings.Cut(k, " ")
		w.Routes = append(w.Routes, routeStats(method, route, s.routes[k]))
	}
	if rest != nil {
		w.Routes = append(w.Routes, routeStats("*", Others, rest))
	}
	s.reset(now)
	return w
}

func routeStats(method, route string, g *group) RouteStats {
	r := RouteStats{Method: method, Route: route, Requests: g.count, Errors: g.errors, Hist: g.hist}
	r.P50MS, r.P95MS = g.quantiles()
	return r
}

// StatusClass devolve a classe do status, como "2xx".
func StatusClass(status int) string {
	return fmt.Sprintf("%dxx", status/100)
}
