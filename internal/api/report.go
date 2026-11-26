package api

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/MiguelDev-GH/omni/internal/alerts"
	"github.com/MiguelDev-GH/omni/internal/connstats"
	"github.com/MiguelDev-GH/omni/internal/dbsize"
	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/mailqueue"
	"github.com/MiguelDev-GH/omni/internal/poller"
	"github.com/MiguelDev-GH/omni/internal/sniffer"
	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/MiguelDev-GH/omni/internal/sysmetrics"
	"github.com/MiguelDev-GH/omni/internal/target"
)

const (
	// reportPoints é quantos pontos, no máximo, cada gráfico do relatório tem.
	reportPoints = 300
	// reportMinStep é a menor janela do sniffer.
	reportMinStep = 10 * time.Second
	// reportTopSources é quantas origens cada faixa de tráfego guarda.
	reportTopSources = 10
	// reportAlertRows é quantas mudanças de nível, as mais graves, o
	// relatório lista. Os totais vêm do resumo, que não tem limite.
	reportAlertRows = 100
)

type ReportStore interface {
	GetTarget(ctx context.Context, id int64) (target.Target, error)
	ChecksBetween(ctx context.Context, targetID int64, from, to time.Time) ([]poller.Result, error)
	TrafficBetween(ctx context.Context, from, to time.Time) ([]sniffer.Window, error)
	HTTPBetween(ctx context.Context, from, to time.Time) ([]l7.HTTPWindow, error)
	DBBetween(ctx context.Context, from, to time.Time) ([]l7.DBWindow, error)
	SystemBetween(ctx context.Context, from, to time.Time, step time.Duration) ([]sysmetrics.Snapshot, error)
	DNSBetween(ctx context.Context, from, to time.Time) ([]l7.DNSWindow, error)
	SMTPBetween(ctx context.Context, from, to time.Time) ([]l7.SMTPWindow, error)
	QueueBetween(ctx context.Context, from, to time.Time, step time.Duration) ([]mailqueue.Snapshot, error)
	ConnectionsBetween(ctx context.Context, from, to time.Time, step time.Duration) ([]connstats.Snapshot, error)
	DBSizesBetween(ctx context.Context, from, to time.Time, step time.Duration) ([]dbsize.Sample, error)
	WorstAlertsBetween(ctx context.Context, from, to time.Time, limit int) ([]alerts.Alert, error)
	AlertSummaryBetween(ctx context.Context, from, to time.Time) ([]store.AlertRuleSummary, error)
	ListScans(ctx context.Context, targetID int64) ([]store.Scan, error)
	ScanFindings(ctx context.Context, scanID int64) ([]findings.Finding, error)
}

// PortUptime resume as verificações de uma porta no intervalo.
type PortUptime struct {
	Port         int     `json:"porta"`
	Kind         string  `json:"tipo"`
	Checks       int     `json:"verificacoes"`
	Up           int     `json:"no_ar"`
	Percent      float64 `json:"percentual"`
	AvgLatencyMS float64 `json:"latencia_media_ms"`
}

// ReportScan é uma varredura do intervalo com os achados.
type ReportScan struct {
	store.Scan
	Findings []findings.Finding `json:"lista_achados"`
}

// Report é a resposta de GET /api/alvos/{id}/relatorio.
type Report struct {
	Target   target.Target `json:"alvo"`
	From     time.Time     `json:"desde"`
	To       time.Time     `json:"ate"`
	StepSecs int64         `json:"passo_s"`
	// AlertCounts conta todas as mudanças de nível do intervalo pelo nível de
	// chegada; AlertSummary faz o mesmo por regra.
	AlertCounts  map[alerts.Level]int     `json:"contagem_alertas"`
	AlertSummary []store.AlertRuleSummary `json:"resumo_alertas"`
	// Alerts traz até reportAlertRows mudanças, as mais graves primeiro.
	Alerts  []alerts.Alert        `json:"alertas"`
	Uptime  []PortUptime          `json:"disponibilidade"`
	Checks  []poller.Result       `json:"verificacoes"`
	Traffic []sniffer.Window      `json:"trafego"`
	HTTP    []l7.HTTPWindow       `json:"http"`
	DB      []l7.DBWindow         `json:"banco"`
	System  []sysmetrics.Snapshot `json:"sistema"`
	DNS     []l7.DNSWindow        `json:"dns"`
	SMTP    []l7.SMTPWindow       `json:"smtp"`
	Queue   []mailqueue.Snapshot  `json:"fila_smtp"`
	Conns   []connstats.Snapshot  `json:"conexoes"`
	DBSizes []dbsize.Sample       `json:"tamanho_banco"`
	Scans   []ReportScan          `json:"varreduras"`
}

// reportStep divide o intervalo em até reportPoints faixas, múltiplas de 10 s.
func reportStep(window time.Duration) time.Duration {
	step := (window + reportPoints - 1) / reportPoints
	return max(reportMinStep, (step+reportMinStep-1)/reportMinStep*reportMinStep)
}

// ReportHandler responde GET /api/alvos/{id}/relatorio?desde=&ate= com o que o
// PDF precisa: alertas, disponibilidade, séries dos gráficos agrupadas em até
// 300 pontos e as varreduras do alvo criadas no intervalo. Tráfego, HTTP,
// banco, DNS, SMTP, conexões e sistema são do servidor inteiro, não só do
// alvo. Intervalo máximo de
// 168 h; sem parâmetros, as últimas 24 h.
func ReportHandler(rs ReportStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		from, to, ok := parseRange(w, r, 24*time.Hour, ReportMaxWindow)
		if !ok {
			return
		}
		rep, err := buildReport(r.Context(), rs, id, from, to)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, rep)
	})
}

func buildReport(ctx context.Context, rs ReportStore, id int64, from, to time.Time) (Report, error) {
	t, err := rs.GetTarget(ctx, id)
	if err != nil {
		return Report{}, err
	}
	step := reportStep(to.Sub(from))
	rep := Report{
		Target: t, From: from.UTC(), To: to.UTC(), StepSecs: int64(step / time.Second),
		AlertCounts: map[alerts.Level]int{alerts.Green: 0, alerts.Yellow: 0, alerts.Red: 0},
	}

	if rep.AlertSummary, err = rs.AlertSummaryBetween(ctx, from, to); err != nil {
		return rep, err
	}
	for _, r := range rep.AlertSummary {
		for lvl, n := range r.Counts {
			rep.AlertCounts[lvl] += n
		}
	}
	if rep.Alerts, err = rs.WorstAlertsBetween(ctx, from, to, reportAlertRows); err != nil {
		return rep, err
	}

	checks, err := rs.ChecksBetween(ctx, id, from, to)
	if err != nil {
		return rep, err
	}
	rep.Uptime = uptime(checks)
	rep.Checks = bucketChecks(checks, from, step)

	traffic, err := rs.TrafficBetween(ctx, from, to)
	if err != nil {
		return rep, err
	}
	rep.Traffic = bucketTraffic(traffic, from, step)

	httpWins, err := rs.HTTPBetween(ctx, from, to)
	if err != nil {
		return rep, err
	}
	rep.HTTP = bucketHTTP(httpWins, from, step)

	dbWins, err := rs.DBBetween(ctx, from, to)
	if err != nil {
		return rep, err
	}
	rep.DB = bucketDB(dbWins, from, step)

	if rep.System, err = rs.SystemBetween(ctx, from, to, step); err != nil {
		return rep, err
	}

	dnsWins, err := rs.DNSBetween(ctx, from, to)
	if err != nil {
		return rep, err
	}
	rep.DNS = bucketDNS(dnsWins, from, step)

	smtpWins, err := rs.SMTPBetween(ctx, from, to)
	if err != nil {
		return rep, err
	}
	rep.SMTP = bucketSMTP(smtpWins, from, step)

	if rep.Queue, err = rs.QueueBetween(ctx, from, to, step); err != nil {
		return rep, err
	}
	if rep.Conns, err = rs.ConnectionsBetween(ctx, from, to, step); err != nil {
		return rep, err
	}
	if rep.DBSizes, err = rs.DBSizesBetween(ctx, from, to, step); err != nil {
		return rep, err
	}

	scans, err := rs.ListScans(ctx, id)
	if err != nil {
		return rep, err
	}
	rep.Scans = []ReportScan{}
	for _, s := range scans {
		if s.CreatedAt.Before(from) || !s.CreatedAt.Before(to) {
			continue
		}
		list, err := rs.ScanFindings(ctx, s.ID)
		if err != nil {
			return rep, err
		}
		rep.Scans = append(rep.Scans, ReportScan{Scan: s, Findings: list})
	}
	sort.Slice(rep.Scans, func(i, j int) bool { return rep.Scans[i].CreatedAt.Before(rep.Scans[j].CreatedAt) })
	return rep, nil
}

func slot(at, from time.Time, step time.Duration) int64 { return int64(at.Sub(from) / step) }

func uptime(checks []poller.Result) []PortUptime {
	type key struct {
		port int
		kind string
	}
	agg := map[key]*PortUptime{}
	for _, c := range checks {
		k := key{c.Port, c.Kind}
		u := agg[k]
		if u == nil {
			u = &PortUptime{Port: c.Port, Kind: c.Kind}
			agg[k] = u
		}
		u.Checks++
		if c.Up {
			u.Up++
		}
		u.AvgLatencyMS += c.LatencyMS
	}
	list := make([]PortUptime, 0, len(agg))
	for _, u := range agg {
		u.Percent = 100 * float64(u.Up) / float64(u.Checks)
		u.AvgLatencyMS /= float64(u.Checks)
		list = append(list, *u)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Port != list[j].Port {
			return list[i].Port < list[j].Port
		}
		return list[i].Kind < list[j].Kind
	})
	return list
}

// bucketChecks junta as verificações de cada porta por faixa: a faixa fica
// fora do ar se qualquer verificação nela falhou, com a latência média e o
// último erro.
func bucketChecks(checks []poller.Result, from time.Time, step time.Duration) []poller.Result {
	type key struct {
		port int
		kind string
		slot int64
	}
	type acc struct {
		r     poller.Result
		n     int
		total float64
	}
	idx := map[key]*acc{}
	order := []key{}
	for _, c := range checks {
		k := key{c.Port, c.Kind, slot(c.At, from, step)}
		a := idx[k]
		if a == nil {
			a = &acc{r: c}
			a.r.Up = true
			idx[k] = a
			order = append(order, k)
		}
		a.n++
		a.total += c.LatencyMS
		if !c.Up {
			a.r.Up = false
			a.r.Err = c.Err
			a.r.StatusCode = c.StatusCode
		}
	}
	list := make([]poller.Result, 0, len(order))
	for _, k := range order {
		a := idx[k]
		a.r.LatencyMS = a.total / float64(a.n)
		list = append(list, a.r)
	}
	return list
}

func addCounts(dst, src map[string]uint64) {
	for k, v := range src {
		dst[k] += v
	}
}

// bucketTraffic soma bytes, pacotes e protocolos por faixa e junta as origens,
// ficando com as maiores.
func bucketTraffic(list []sniffer.Window, from time.Time, step time.Duration) []sniffer.Window {
	out := []sniffer.Window{}
	cur := int64(-1)
	var sources map[string]*sniffer.SourceCount
	flush := func() {
		if len(out) == 0 {
			return
		}
		top := make([]sniffer.SourceCount, 0, len(sources))
		for _, s := range sources {
			top = append(top, *s)
		}
		sort.Slice(top, func(i, j int) bool {
			if top[i].Bytes != top[j].Bytes {
				return top[i].Bytes > top[j].Bytes
			}
			return top[i].IP < top[j].IP
		})
		if len(top) > reportTopSources {
			top = top[:reportTopSources]
		}
		out[len(out)-1].TopSources = top
	}
	for _, w := range list {
		if s := slot(w.Start, from, step); s != cur {
			flush()
			cur = s
			sources = map[string]*sniffer.SourceCount{}
			out = append(out, sniffer.Window{Start: w.Start, ByProtocol: map[string]uint64{}})
		}
		b := &out[len(out)-1]
		b.Bytes += w.Bytes
		b.Packets += w.Packets
		b.Dropped += w.Dropped
		addCounts(b.ByProtocol, w.ByProtocol)
		for _, src := range w.TopSources {
			if sources[src.IP] == nil {
				sources[src.IP] = &sniffer.SourceCount{IP: src.IP}
			}
			sources[src.IP].Bytes += src.Bytes
			sources[src.IP].Packets += src.Packets
		}
	}
	flush()
	return out
}

// bucketHTTP soma requisições, status e códigos por faixa. Os percentis viram a média
// dos percentis das janelas ponderada pelas requisições, uma aproximação.
func bucketHTTP(list []l7.HTTPWindow, from time.Time, step time.Duration) []l7.HTTPWindow {
	out := []l7.HTTPWindow{}
	cur := int64(-1)
	for _, w := range list {
		if s := slot(w.Start, from, step); s != cur {
			cur = s
			out = append(out, l7.HTTPWindow{Start: w.Start, ByStatus: map[string]uint64{}, ByCode: map[string]uint64{}, Routes: []l7.RouteStats{}})
		}
		b := &out[len(out)-1]
		b.P50MS = weighted(b.P50MS, b.Requests, w.P50MS, w.Requests)
		b.P95MS = weighted(b.P95MS, b.Requests, w.P95MS, w.Requests)
		b.Requests += w.Requests
		addCounts(b.ByStatus, w.ByStatus)
		addCounts(b.ByCode, w.ByCode)
	}
	return out
}

// bucketDB faz o mesmo que bucketHTTP para as janelas do banco.
func bucketDB(list []l7.DBWindow, from time.Time, step time.Duration) []l7.DBWindow {
	out := []l7.DBWindow{}
	cur := int64(-1)
	for _, w := range list {
		if s := slot(w.Start, from, step); s != cur {
			cur = s
			out = append(out, l7.DBWindow{Start: w.Start, ByKind: map[string]uint64{}, Top: []l7.QueryStats{}})
		}
		b := &out[len(out)-1]
		b.P50MS = weighted(b.P50MS, b.Queries, w.P50MS, w.Queries)
		b.P95MS = weighted(b.P95MS, b.Queries, w.P95MS, w.Queries)
		b.Queries += w.Queries
		b.Errors += w.Errors
		b.TxFailed += w.TxFailed
		addCounts(b.ByKind, w.ByKind)
	}
	return out
}

// bucketDNS faz o mesmo que bucketHTTP para as janelas DNS.
func bucketDNS(list []l7.DNSWindow, from time.Time, step time.Duration) []l7.DNSWindow {
	out := []l7.DNSWindow{}
	cur := int64(-1)
	for _, w := range list {
		if s := slot(w.Start, from, step); s != cur {
			cur = s
			out = append(out, l7.DNSWindow{Start: w.Start, ByRCode: map[string]uint64{}})
		}
		b := &out[len(out)-1]
		b.P50MS = weighted(b.P50MS, b.Queries, w.P50MS, w.Queries)
		b.P95MS = weighted(b.P95MS, b.Queries, w.P95MS, w.Queries)
		b.Queries += w.Queries
		b.Failures += w.Failures
		addCounts(b.ByRCode, w.ByRCode)
	}
	return out
}

// bucketSMTP faz o mesmo que bucketHTTP para as janelas SMTP; os percentis
// são ponderados pelas tentativas.
func bucketSMTP(list []l7.SMTPWindow, from time.Time, step time.Duration) []l7.SMTPWindow {
	out := []l7.SMTPWindow{}
	cur := int64(-1)
	for _, w := range list {
		if s := slot(w.Start, from, step); s != cur {
			cur = s
			out = append(out, l7.SMTPWindow{Start: w.Start, ByCode: map[string]uint64{}})
		}
		b := &out[len(out)-1]
		b.P50MS = weighted(b.P50MS, b.Attempts, w.P50MS, w.Attempts)
		b.P95MS = weighted(b.P95MS, b.Attempts, w.P95MS, w.Attempts)
		b.Attempts += w.Attempts
		b.Delivered += w.Delivered
		b.Rejected += w.Rejected
		b.Aborted += w.Aborted
		addCounts(b.ByCode, w.ByCode)
	}
	return out
}

func weighted(a float64, na uint64, b float64, nb uint64) float64 {
	if na+nb == 0 {
		return 0
	}
	return (a*float64(na) + b*float64(nb)) / float64(na+nb)
}
