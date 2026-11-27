package store

import (
	"context"
	"time"

	"github.com/MiguelDev-GH/omni/internal/dbsize"
)

// AddDBSizes grava uma medição por banco; a mesma hora e banco, em
// milissegundos, substitui a anterior.
func (s *Store) AddDBSizes(ctx context.Context, list []dbsize.Sample) error {
	if len(list) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, m := range list {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO db_sizes (time_ms, name, bytes) VALUES (?, ?, ?)`,
			m.At.UnixMilli(), m.Name, m.Bytes); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DBSizesSince devolve as medições com hora >= since, em ordem cronológica
// e, na mesma hora, pelo nome do banco.
func (s *Store) DBSizesSince(ctx context.Context, since time.Time) ([]dbsize.Sample, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT time_ms, name, bytes FROM db_sizes WHERE time_ms >= ? ORDER BY time_ms, name LIMIT ?`,
		since.UnixMilli(), maxGroupRows)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []dbsize.Sample{}
	for rows.Next() {
		var (
			m  dbsize.Sample
			ms int64
		)
		if err := rows.Scan(&ms, &m.Name, &m.Bytes); err != nil {
			return nil, err
		}
		m.At = time.UnixMilli(ms).UTC()
		list = append(list, m)
	}
	return list, rows.Err()
}

// DBSizesBetween devolve, por banco, o maior tamanho medido em cada intervalo
// de step contado a partir de from, com from <= hora < to. A hora de cada
// medição é o início do intervalo.
func (s *Store) DBSizesBetween(ctx context.Context, from, to time.Time, step time.Duration) ([]dbsize.Sample, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT ?1 + (time_ms - ?1) / ?3 * ?3 AS slot, name, MAX(bytes) FROM db_sizes
		 WHERE time_ms >= ?1 AND time_ms < ?2 GROUP BY slot, name ORDER BY slot, name`,
		from.UnixMilli(), to.UnixMilli(), max(step.Milliseconds(), 1))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []dbsize.Sample{}
	for rows.Next() {
		var (
			m  dbsize.Sample
			ms int64
		)
		if err := rows.Scan(&ms, &m.Name, &m.Bytes); err != nil {
			return nil, err
		}
		m.At = time.UnixMilli(ms).UTC()
		list = append(list, m)
	}
	return list, rows.Err()
}

// PruneDBSizes apaga as medições anteriores a before.
func (s *Store) PruneDBSizes(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM db_sizes WHERE time_ms < ?", before.UnixMilli())
	return err
}
