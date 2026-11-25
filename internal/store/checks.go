package store

import (
	"context"
	"time"

	"github.com/MiguelDev-GH/omni/internal/poller"
)

// InsertCheckResult grava o resultado de uma verificação; At é guardado em milissegundos.
func (s *Store) InsertCheckResult(ctx context.Context, r poller.Result) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO check_results (target_id, port, kind, up, status_code, latency_ms, error, checked_at_ms, degraded, detail, starttls)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.TargetID, r.Port, r.Kind, r.Up, r.StatusCode, r.LatencyMS, r.Err, r.At.UnixMilli(), r.Degraded, r.Detail, startTLSColumn(r.StartTLS))
	return err
}

// CheckHistory devolve, para cada porta e tipo de verificação, os perPort
// resultados mais recentes com At >= since, todos em ordem cronológica
// crescente; sem resultados, devolve slice vazio. O limite por porta impede
// que um alvo com muitas portas empurre o histórico das outras para fora.
func (s *Store) CheckHistory(ctx context.Context, targetID int64, since time.Time, perPort int) ([]poller.Result, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT target_id, port, kind, up, status_code, latency_ms, error, checked_at_ms, degraded, detail, starttls FROM (
		   SELECT *, ROW_NUMBER() OVER (PARTITION BY port, kind ORDER BY checked_at_ms DESC, id DESC) AS n
		   FROM check_results WHERE target_id = ? AND checked_at_ms >= ?
		 ) WHERE n <= ?
		 ORDER BY checked_at_ms DESC, id DESC`,
		targetID, since.UnixMilli(), perPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	list := []poller.Result{}
	for rows.Next() {
		r, err := scanCheck(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}

// PruneChecks apaga os resultados de verificação anteriores a before.
func (s *Store) PruneChecks(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM check_results WHERE checked_at_ms < ?`, before.UnixMilli())
	return err
}

// ChecksBetween devolve os resultados do alvo com from <= hora < to, em ordem
// cronológica, até maxGroupRows.
func (s *Store) ChecksBetween(ctx context.Context, targetID int64, from, to time.Time) ([]poller.Result, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT target_id, port, kind, up, status_code, latency_ms, error, checked_at_ms, degraded, detail, starttls FROM check_results
		 WHERE target_id = ? AND checked_at_ms >= ? AND checked_at_ms < ?
		 ORDER BY checked_at_ms, id LIMIT ?`,
		targetID, from.UnixMilli(), to.UnixMilli(), maxGroupRows)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []poller.Result{}
	for rows.Next() {
		r, err := scanCheck(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

// startTLSColumn guarda *bool como -1 (não se aplica), 0 ou 1.
func startTLSColumn(v *bool) int {
	switch {
	case v == nil:
		return -1
	case *v:
		return 1
	default:
		return 0
	}
}

// scanCheck lê uma linha com as colunas de InsertCheckResult.
func scanCheck(rows interface{ Scan(...any) error }) (poller.Result, error) {
	var (
		r        poller.Result
		ms       int64
		startTLS int
	)
	if err := rows.Scan(&r.TargetID, &r.Port, &r.Kind, &r.Up, &r.StatusCode, &r.LatencyMS, &r.Err, &ms, &r.Degraded, &r.Detail, &startTLS); err != nil {
		return poller.Result{}, err
	}
	r.At = time.UnixMilli(ms).UTC()
	if startTLS >= 0 {
		v := startTLS == 1
		r.StartTLS = &v
	}
	return r, nil
}
