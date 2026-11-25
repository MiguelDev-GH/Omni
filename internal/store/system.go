package store

import (
	"context"
	"time"

	"github.com/MiguelDev-GH/omni/internal/sysmetrics"
)

// AddSystemSample grava uma leitura de CPU, memória e disco; uma leitura com a
// mesma hora, em milissegundos, substitui a anterior.
func (s *Store) AddSystemSample(ctx context.Context, snap sysmetrics.Snapshot) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO system_metrics (time_ms, cpu, memory, disk) VALUES (?, ?, ?, ?)`,
		snap.At.UnixMilli(), snap.CPUPercent, snap.MemPercent, snap.DiskPercent)
	return err
}

// SystemSince devolve as limit leituras mais recentes com hora >= since, em
// ordem cronológica crescente. Com step > 0, as leituras viram uma média por
// intervalo de step contado a partir de since, com a hora da primeira leitura
// do intervalo. Sem leituras, devolve slice vazio.
func (s *Store) SystemSince(ctx context.Context, since time.Time, step time.Duration, limit int) ([]sysmetrics.Snapshot, error) {
	stepMs := step.Milliseconds()
	if stepMs <= 0 {
		// Cada leitura no próprio grupo: time_ms é a chave primária.
		stepMs = 1
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT MIN(time_ms), AVG(cpu), AVG(memory), AVG(disk)
		 FROM system_metrics WHERE time_ms >= ?1
		 GROUP BY (time_ms - ?1) / ?2 ORDER BY 1 DESC LIMIT ?3`,
		since.UnixMilli(), stepMs, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	list := []sysmetrics.Snapshot{}
	for rows.Next() {
		var (
			snap sysmetrics.Snapshot
			ms   int64
		)
		if err := rows.Scan(&ms, &snap.CPUPercent, &snap.MemPercent, &snap.DiskPercent); err != nil {
			return nil, err
		}
		snap.At = time.UnixMilli(ms).UTC()
		list = append(list, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	reverse(list)
	return list, nil
}

// PruneSystem apaga as leituras anteriores a before.
func (s *Store) PruneSystem(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM system_metrics WHERE time_ms < ?", before.UnixMilli())
	return err
}

// SystemBetween devolve as leituras com from <= hora < to, em ordem
// cronológica, com uma média por intervalo de step contado a partir de from.
func (s *Store) SystemBetween(ctx context.Context, from, to time.Time, step time.Duration) ([]sysmetrics.Snapshot, error) {
	stepMs := max(step.Milliseconds(), 1)
	rows, err := s.db.QueryContext(ctx,
		`SELECT MIN(time_ms), AVG(cpu), AVG(memory), AVG(disk)
		 FROM system_metrics WHERE time_ms >= ?1 AND time_ms < ?2
		 GROUP BY (time_ms - ?1) / ?3 ORDER BY 1`,
		from.UnixMilli(), to.UnixMilli(), stepMs)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []sysmetrics.Snapshot{}
	for rows.Next() {
		var (
			snap sysmetrics.Snapshot
			ms   int64
		)
		if err := rows.Scan(&ms, &snap.CPUPercent, &snap.MemPercent, &snap.DiskPercent); err != nil {
			return nil, err
		}
		snap.At = time.UnixMilli(ms).UTC()
		list = append(list, snap)
	}
	return list, rows.Err()
}
