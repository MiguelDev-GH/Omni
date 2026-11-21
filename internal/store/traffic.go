package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/MiguelDev-GH/omni/internal/sniffer"
)

// InsertTrafficWindow grava uma janela do sniffer; uma janela com o mesmo
// início, em milissegundos, substitui a anterior.
func (s *Store) InsertTrafficWindow(ctx context.Context, w sniffer.Window) error {
	protocols, err := json.Marshal(w.ByProtocol)
	if err != nil {
		return err
	}
	sources, err := json.Marshal(w.TopSources)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO traffic_windows (start_ms, bytes, packets, dropped, protocols, top_sources)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		w.Start.UnixMilli(), int64(w.Bytes), int64(w.Packets), int64(w.Dropped), string(protocols), string(sources))
	return err
}

// TrafficSince devolve as limit janelas mais recentes com Start >= since, em
// ordem cronológica crescente; sem janelas, devolve slice vazio.
func (s *Store) TrafficSince(ctx context.Context, since time.Time, limit int) ([]sniffer.Window, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT start_ms, bytes, packets, dropped, protocols, top_sources
		 FROM traffic_windows WHERE start_ms >= ? ORDER BY start_ms DESC LIMIT ?`,
		since.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	list := []sniffer.Window{}
	for rows.Next() {
		var (
			w                  sniffer.Window
			ms                 int64
			bytes, pkts, drops int64
			protocols, sources string
		)
		if err := rows.Scan(&ms, &bytes, &pkts, &drops, &protocols, &sources); err != nil {
			return nil, err
		}
		w.Start = time.UnixMilli(ms).UTC()
		w.Bytes, w.Packets, w.Dropped = uint64(bytes), uint64(pkts), uint64(drops)
		if err := json.Unmarshal([]byte(protocols), &w.ByProtocol); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(sources), &w.TopSources); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}

// PruneTraffic apaga as janelas que começaram antes de before.
func (s *Store) PruneTraffic(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM traffic_windows WHERE start_ms < ?", before.UnixMilli())
	return err
}
