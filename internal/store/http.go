package store

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
)

// maxGroupRows limita as linhas lidas ao somar rotas ou consultas de um período.
const maxGroupRows = 200_000

// InsertHTTPWindow grava uma janela HTTP e as rotas dela; uma janela com o
// mesmo início, em milissegundos, substitui a anterior.
func (s *Store) InsertHTTPWindow(ctx context.Context, w l7.HTTPWindow) error {
	byStatus, err := json.Marshal(w.ByStatus)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	ms := w.Start.UnixMilli()
	if _, err := tx.ExecContext(ctx, "DELETE FROM http_routes WHERE start_ms = ?", ms); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO http_windows (start_ms, requests, by_status, p50_ms, p95_ms) VALUES (?, ?, ?, ?, ?)`,
		ms, int64(w.Requests), string(byStatus), w.P50MS, w.P95MS); err != nil {
		return err
	}
	for _, r := range w.Routes {
		hist, err := json.Marshal(trimHist(r.Hist))
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO http_routes (start_ms, method, route, requests, errors, p50_ms, p95_ms, hist)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			ms, r.Method, r.Route, int64(r.Requests), int64(r.Errors), r.P50MS, r.P95MS, string(hist)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// HTTPSince devolve as limit janelas HTTP mais recentes com início >= since,
// em ordem cronológica e sem as rotas.
func (s *Store) HTTPSince(ctx context.Context, since time.Time, limit int) ([]l7.HTTPWindow, error) {
	list, err := s.queryHTTP(ctx,
		`SELECT start_ms, requests, by_status, p50_ms, p95_ms FROM http_windows
		 WHERE start_ms >= ? ORDER BY start_ms DESC LIMIT ?`, since.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	reverse(list)
	return list, nil
}

// HTTPBetween devolve as janelas HTTP com from <= início < to, em ordem
// cronológica e sem as rotas, até maxGroupRows.
func (s *Store) HTTPBetween(ctx context.Context, from, to time.Time) ([]l7.HTTPWindow, error) {
	return s.queryHTTP(ctx,
		`SELECT start_ms, requests, by_status, p50_ms, p95_ms FROM http_windows
		 WHERE start_ms >= ? AND start_ms < ? ORDER BY start_ms LIMIT ?`,
		from.UnixMilli(), to.UnixMilli(), maxGroupRows)
}

func (s *Store) queryHTTP(ctx context.Context, query string, args ...any) ([]l7.HTTPWindow, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []l7.HTTPWindow{}
	for rows.Next() {
		var (
			w        l7.HTTPWindow
			ms, reqs int64
			byStatus string
		)
		if err := rows.Scan(&ms, &reqs, &byStatus, &w.P50MS, &w.P95MS); err != nil {
			return nil, err
		}
		w.Start = time.UnixMilli(ms).UTC()
		w.Requests = uint64(reqs)
		w.Routes = []l7.RouteStats{}
		if err := json.Unmarshal([]byte(byStatus), &w.ByStatus); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

// HTTPRoutesSince soma as rotas das janelas com início >= since e devolve as
// limit com mais requisições. Os percentis vêm dos histogramas somados, com
// a precisão das faixas (25%).
func (s *Store) HTTPRoutesSince(ctx context.Context, since time.Time, limit int) ([]l7.RouteStats, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT method, route, requests, errors, hist FROM http_routes WHERE start_ms >= ? LIMIT ?`,
		since.UnixMilli(), maxGroupRows)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	type key struct{ method, route string }
	sum := map[key]*l7.RouteStats{}
	for rows.Next() {
		var (
			k          key
			reqs, errs int64
			histJSON   string
			hist       l7.Histogram
		)
		if err := rows.Scan(&k.method, &k.route, &reqs, &errs, &histJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(histJSON), &hist); err != nil {
			return nil, err
		}
		r := sum[k]
		if r == nil {
			r = &l7.RouteStats{Method: k.method, Route: k.route}
			sum[k] = r
		}
		r.Requests += uint64(reqs)
		r.Errors += uint64(errs)
		r.Hist = r.Hist.Merge(hist)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	list := make([]l7.RouteStats, 0, len(sum))
	for _, r := range sum {
		r.P50MS, r.P95MS = r.Hist.Quantile(0.5), r.Hist.Quantile(0.95)
		list = append(list, *r)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Requests != list[j].Requests {
			return list[i].Requests > list[j].Requests
		}
		return list[i].Method+list[i].Route < list[j].Method+list[j].Route
	})
	if len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

// trimHist tira os zeros do fim do histograma, que o Merge recompõe.
func trimHist(h l7.Histogram) l7.Histogram {
	n := len(h)
	for n > 0 && h[n-1] == 0 {
		n--
	}
	if n == 0 {
		return l7.Histogram{}
	}
	return h[:n]
}

func reverse[T any](list []T) {
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
}
