package store

import (
	"context"
	"encoding/json"
	"time"

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
	rows, err := s.db.QueryContext(ctx,
		`SELECT start_ms, queries, failures, by_rcode, p50_ms, p95_ms FROM dns_windows
		 WHERE start_ms >= ? ORDER BY start_ms DESC LIMIT ?`, since.UnixMilli(), limit)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	reverse(list)
	return list, nil
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
	rows, err := s.db.QueryContext(ctx,
		`SELECT start_ms, attempts, delivered, rejected, aborted, by_code, p50_ms, p95_ms FROM smtp_windows
		 WHERE start_ms >= ? ORDER BY start_ms DESC LIMIT ?`, since.UnixMilli(), limit)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	reverse(list)
	return list, nil
}

// nonNilCounts troca um mapa nil por um vazio, para o JSON sair "{}".
func nonNilCounts(m map[string]uint64) map[string]uint64 {
	if m == nil {
		return map[string]uint64{}
	}
	return m
}
