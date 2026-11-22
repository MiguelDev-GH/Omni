package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// SaveReport grava o relatório estruturado de uma ferramenta na varredura,
// trocando o anterior se houver.
func (s *Store) SaveReport(ctx context.Context, scanID int64, tool string, report json.RawMessage) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO scan_reports (scan_id, tool, report) VALUES (?, ?, ?)",
		scanID, tool, string(report))
	return err
}

// Report devolve o relatório de uma ferramenta na varredura; ok é false se
// não houver (varredura antiga, ferramenta sem relatório ou falha ao montá-lo).
func (s *Store) Report(ctx context.Context, scanID int64, tool string) (json.RawMessage, bool, error) {
	var report string
	err := s.db.QueryRowContext(ctx,
		"SELECT report FROM scan_reports WHERE scan_id = ? AND tool = ?", scanID, tool).Scan(&report)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return json.RawMessage(report), true, nil
}
