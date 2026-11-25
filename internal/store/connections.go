package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/MiguelDev-GH/omni/internal/connstats"
)

// AddConnSnapshot grava uma linha por porta da coleta; uma coleta com a
// mesma hora, em milissegundos, substitui a anterior.
func (s *Store) AddConnSnapshot(ctx context.Context, snap connstats.Snapshot) error {
	if len(snap.Ports) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	ms := snap.At.UnixMilli()
	for _, p := range snap.Ports {
		states, err := json.Marshal(p.States)
		if err != nil {
			return err
		}
		clients, err := json.Marshal(p.Clients)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO conn_samples (time_ms, port, kind, established, states, clients) VALUES (?, ?, ?, ?, ?, ?)`,
			ms, p.Port, p.Kind, p.Established, string(states), string(clients)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ConnectionsSince devolve as limit coletas mais recentes com hora >= since,
// em ordem cronológica e com as portas na ordem em que foram gravadas.
func (s *Store) ConnectionsSince(ctx context.Context, since time.Time, limit int) ([]connstats.Snapshot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT time_ms, port, kind, established, states, clients FROM conn_samples
		 WHERE time_ms IN (SELECT DISTINCT time_ms FROM conn_samples WHERE time_ms >= ? ORDER BY time_ms DESC LIMIT ?)
		 ORDER BY time_ms, rowid`,
		since.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []connstats.Snapshot{}
	for rows.Next() {
		var (
			ms              int64
			p               connstats.PortConns
			states, clients string
		)
		if err := rows.Scan(&ms, &p.Port, &p.Kind, &p.Established, &states, &clients); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(states), &p.States); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(clients), &p.Clients); err != nil {
			return nil, err
		}
		at := time.UnixMilli(ms).UTC()
		if n := len(list); n == 0 || !list[n-1].At.Equal(at) {
			list = append(list, connstats.Snapshot{At: at, Ports: []connstats.PortConns{}})
		}
		last := &list[len(list)-1]
		last.Ports = append(last.Ports, p)
	}
	return list, rows.Err()
}

// PruneConnections apaga as coletas anteriores a before.
func (s *Store) PruneConnections(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM conn_samples WHERE time_ms < ?", before.UnixMilli())
	return err
}
