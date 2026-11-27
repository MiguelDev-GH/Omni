package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/MiguelDev-GH/omni/internal/mailqueue"
	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
)

// InsertDNSWindow grava uma janela DNS; uma janela com o mesmo início, em
// milissegundos, substitui a anterior.
func (s *Store) InsertDNSWindow(ctx context.Context, w l7.DNSWindow) error {
	byRCode, err := json.Marshal(nonNilCounts(w.ByRCode))
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO dns_windows (start_ms, queries, failures, by_rcode, p50_ms, p95_ms) VALUES (?, ?, ?, ?, ?, ?)`,
		w.Start.UnixMilli(), int64(w.Queries), int64(w.Failures), string(byRCode), w.P50MS, w.P95MS)
	return err
}

// DNSSince devolve as limit janelas DNS mais recentes com início >= since,
// em ordem cronológica.
func (s *Store) DNSSince(ctx context.Context, since time.Time, limit int) ([]l7.DNSWindow, error) {
	list, err := s.queryDNS(ctx,
		`SELECT start_ms, queries, failures, by_rcode, p50_ms, p95_ms FROM dns_windows
		 WHERE start_ms >= ? ORDER BY start_ms DESC LIMIT ?`, since.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	reverse(list)
	return list, nil
}

// DNSBetween devolve as janelas DNS com from <= início < to, em ordem
// cronológica, até maxGroupRows.
func (s *Store) DNSBetween(ctx context.Context, from, to time.Time) ([]l7.DNSWindow, error) {
	return s.queryDNS(ctx,
		`SELECT start_ms, queries, failures, by_rcode, p50_ms, p95_ms FROM dns_windows
		 WHERE start_ms >= ? AND start_ms < ? ORDER BY start_ms LIMIT ?`,
		from.UnixMilli(), to.UnixMilli(), maxGroupRows)
}

func (s *Store) queryDNS(ctx context.Context, query string, args ...any) ([]l7.DNSWindow, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []l7.DNSWindow{}
	for rows.Next() {
		var (
			w                  l7.DNSWindow
			ms, queries, fails int64
			byRCode            string
		)
		if err := rows.Scan(&ms, &queries, &fails, &byRCode, &w.P50MS, &w.P95MS); err != nil {
			return nil, err
		}
		w.Start = time.UnixMilli(ms).UTC()
		w.Queries, w.Failures = uint64(queries), uint64(fails)
		if err := json.Unmarshal([]byte(byRCode), &w.ByRCode); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

// InsertSMTPWindow grava uma janela SMTP; uma janela com o mesmo início, em
// milissegundos, substitui a anterior.
func (s *Store) InsertSMTPWindow(ctx context.Context, w l7.SMTPWindow) error {
	byCode, err := json.Marshal(nonNilCounts(w.ByCode))
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO smtp_windows (start_ms, attempts, delivered, rejected, aborted, by_code, p50_ms, p95_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		w.Start.UnixMilli(), int64(w.Attempts), int64(w.Delivered), int64(w.Rejected), int64(w.Aborted),
		string(byCode), w.P50MS, w.P95MS)
	return err
}

// SMTPSince devolve as limit janelas SMTP mais recentes com início >= since,
// em ordem cronológica.
func (s *Store) SMTPSince(ctx context.Context, since time.Time, limit int) ([]l7.SMTPWindow, error) {
	list, err := s.querySMTP(ctx,
		`SELECT start_ms, attempts, delivered, rejected, aborted, by_code, p50_ms, p95_ms FROM smtp_windows
		 WHERE start_ms >= ? ORDER BY start_ms DESC LIMIT ?`, since.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	reverse(list)
	return list, nil
}

// SMTPBetween devolve as janelas SMTP com from <= início < to, em ordem
// cronológica, até maxGroupRows.
func (s *Store) SMTPBetween(ctx context.Context, from, to time.Time) ([]l7.SMTPWindow, error) {
	return s.querySMTP(ctx,
		`SELECT start_ms, attempts, delivered, rejected, aborted, by_code, p50_ms, p95_ms FROM smtp_windows
		 WHERE start_ms >= ? AND start_ms < ? ORDER BY start_ms LIMIT ?`,
		from.UnixMilli(), to.UnixMilli(), maxGroupRows)
}

func (s *Store) querySMTP(ctx context.Context, query string, args ...any) ([]l7.SMTPWindow, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []l7.SMTPWindow{}
	for rows.Next() {
		var (
			w                                          l7.SMTPWindow
			ms, attempts, delivered, rejected, aborted int64
			byCode                                     string
		)
		if err := rows.Scan(&ms, &attempts, &delivered, &rejected, &aborted, &byCode, &w.P50MS, &w.P95MS); err != nil {
			return nil, err
		}
		w.Start = time.UnixMilli(ms).UTC()
		w.Attempts, w.Delivered, w.Rejected, w.Aborted = uint64(attempts), uint64(delivered), uint64(rejected), uint64(aborted)
		if err := json.Unmarshal([]byte(byCode), &w.ByCode); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

// nonNilCounts troca um mapa nil por um vazio, para o JSON sair "{}".
func nonNilCounts(m map[string]uint64) map[string]uint64 {
	if m == nil {
		return map[string]uint64{}
	}
	return m
}

// AddQueueSnapshot grava uma contagem da fila do SMTP; a mesma hora, em
// milissegundos, substitui a anterior.
func (s *Store) AddQueueSnapshot(ctx context.Context, snap mailqueue.Snapshot) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO smtp_queue (time_ms, messages) VALUES (?, ?)`,
		snap.At.UnixMilli(), snap.Messages)
	return err
}

// QueueSince devolve as limit contagens mais recentes com hora >= since, em
// ordem cronológica.
func (s *Store) QueueSince(ctx context.Context, since time.Time, limit int) ([]mailqueue.Snapshot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT time_ms, messages FROM smtp_queue WHERE time_ms >= ? ORDER BY time_ms DESC LIMIT ?`,
		since.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []mailqueue.Snapshot{}
	for rows.Next() {
		var (
			snap mailqueue.Snapshot
			ms   int64
		)
		if err := rows.Scan(&ms, &snap.Messages); err != nil {
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

// QueueBetween devolve o pico da fila do SMTP em cada intervalo de step
// contado a partir de from, com from <= hora < to, em ordem cronológica.
func (s *Store) QueueBetween(ctx context.Context, from, to time.Time, step time.Duration) ([]mailqueue.Snapshot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT MIN(time_ms), MAX(messages) FROM smtp_queue WHERE time_ms >= ?1 AND time_ms < ?2
		 GROUP BY (time_ms - ?1) / ?3 ORDER BY 1`,
		from.UnixMilli(), to.UnixMilli(), max(step.Milliseconds(), 1))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []mailqueue.Snapshot{}
	for rows.Next() {
		var (
			snap mailqueue.Snapshot
			ms   int64
		)
		if err := rows.Scan(&ms, &snap.Messages); err != nil {
			return nil, err
		}
		snap.At = time.UnixMilli(ms).UTC()
		list = append(list, snap)
	}
	return list, rows.Err()
}

// PruneQueue apaga as contagens da fila anteriores a before.
func (s *Store) PruneQueue(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM smtp_queue WHERE time_ms < ?", before.UnixMilli())
	return err
}
