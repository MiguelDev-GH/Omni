package store

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
)

// InsertDBWindow grava uma janela de consultas ao banco e as consultas dela;
// uma janela com o mesmo início, em milissegundos, substitui a anterior.
func (s *Store) InsertDBWindow(ctx context.Context, w l7.DBWindow) error {
	byKind, err := json.Marshal(w.ByKind)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	ms := w.Start.UnixMilli()
	if _, err := tx.ExecContext(ctx, "DELETE FROM db_queries WHERE start_ms = ?", ms); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO db_windows (start_ms, queries, errors, by_kind, p50_ms, p95_ms, tx_failed) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ms, int64(w.Queries), int64(w.Errors), string(byKind), w.P50MS, w.P95MS, int64(w.TxFailed)); err != nil {
		return err
	}
	for _, q := range w.Top {
		hist, err := json.Marshal(trimHist(q.Hist))
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO db_queries (start_ms, query, kind, count, errors, p50_ms, p95_ms, hist)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			ms, q.Query, q.Kind, int64(q.Count), int64(q.Errors), q.P50MS, q.P95MS, string(hist)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DBSince devolve as limit janelas de consultas mais recentes com início >=
// since, em ordem cronológica e sem as consultas.
func (s *Store) DBSince(ctx context.Context, since time.Time, limit int) ([]l7.DBWindow, error) {
	list, err := s.queryDB(ctx,
		`SELECT start_ms, queries, errors, by_kind, p50_ms, p95_ms, tx_failed FROM db_windows
		 WHERE start_ms >= ? ORDER BY start_ms DESC LIMIT ?`, since.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	reverse(list)
	return list, nil
}

// DBBetween devolve as janelas do banco com from <= início < to, em ordem
// cronológica e sem as consultas, até maxGroupRows.
func (s *Store) DBBetween(ctx context.Context, from, to time.Time) ([]l7.DBWindow, error) {
	return s.queryDB(ctx,
		`SELECT start_ms, queries, errors, by_kind, p50_ms, p95_ms, tx_failed FROM db_windows
		 WHERE start_ms >= ? AND start_ms < ? ORDER BY start_ms LIMIT ?`,
		from.UnixMilli(), to.UnixMilli(), maxGroupRows)
}

func (s *Store) queryDB(ctx context.Context, query string, args ...any) ([]l7.DBWindow, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []l7.DBWindow{}
	for rows.Next() {
		var (
			w                           l7.DBWindow
			ms, queries, errs, txFailed int64
			byKind                      string
		)
		if err := rows.Scan(&ms, &queries, &errs, &byKind, &w.P50MS, &w.P95MS, &txFailed); err != nil {
			return nil, err
		}
		w.Start = time.UnixMilli(ms).UTC()
		w.Queries, w.Errors, w.TxFailed = uint64(queries), uint64(errs), uint64(txFailed)
		w.Top = []l7.QueryStats{}
		if err := json.Unmarshal([]byte(byKind), &w.ByKind); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

// SlowQueriesSince soma as consultas das janelas com início >= since e
// devolve as limit de maior p95. Os percentis vêm dos histogramas somados,
// com a precisão das faixas (25%).
func (s *Store) SlowQueriesSince(ctx context.Context, since time.Time, limit int) ([]l7.QueryStats, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT query, kind, count, errors, hist FROM db_queries WHERE start_ms >= ? LIMIT ?`,
		since.UnixMilli(), maxGroupRows)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	sum := map[string]*l7.QueryStats{}
	for rows.Next() {
		var (
			query, kind, histJSON string
			count, errs           int64
			hist                  l7.Histogram
		)
		if err := rows.Scan(&query, &kind, &count, &errs, &histJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(histJSON), &hist); err != nil {
			return nil, err
		}
		q := sum[query]
		if q == nil {
			q = &l7.QueryStats{Query: query, Kind: kind}
			sum[query] = q
		}
		q.Count += uint64(count)
		q.Errors += uint64(errs)
		q.Hist = q.Hist.Merge(hist)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	list := make([]l7.QueryStats, 0, len(sum))
	for _, q := range sum {
		q.P50MS, q.P95MS = q.Hist.Quantile(0.5), q.Hist.Quantile(0.95)
		list = append(list, *q)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].P95MS != list[j].P95MS {
			return list[i].P95MS > list[j].P95MS
		}
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].Query < list[j].Query
	})
	if len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}
