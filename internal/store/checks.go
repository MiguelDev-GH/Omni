package store

import (
	"context"
	"time"

	"github.com/MiguelDev-GH/omni/internal/poller"
)

// InsertCheckResult grava o resultado de uma verificação; At é guardado em milissegundos.
func (s *Store) InsertCheckResult(ctx context.Context, r poller.Result) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO check_results (target_id, port, kind, up, status_code, latency_ms, error, checked_at_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.TargetID, r.Port, r.Kind, r.Up, r.StatusCode, r.LatencyMS, r.Err, r.At.UnixMilli())
	return err
}

// CheckHistory devolve, para cada porta e tipo de verificação, os perPort
// resultados mais recentes com At >= since, todos em ordem cronológica
// crescente; sem resultados, devolve slice vazio. O limite por porta impede
// que um alvo com muitas portas empurre o histórico das outras para fora.
func (s *Store) CheckHistory(ctx context.Context, targetID int64, since time.Time, perPort int) ([]poller.Result, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT target_id, port, kind, up, status_code, latency_ms, error, checked_at_ms FROM (
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
		var r poller.Result
		var ms int64
		if err := rows.Scan(&r.TargetID, &r.Port, &r.Kind, &r.Up, &r.StatusCode, &r.LatencyMS, &r.Err, &ms); err != nil {
			return nil, err
		}
		r.At = time.UnixMilli(ms).UTC()
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
