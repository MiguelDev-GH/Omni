package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/MiguelDev-GH/omni/internal/capture"
)

const captureColumns = "id, file, started_ms, finished_ms, duration_s, port, packets, bytes, dropped, status, error"

// CreateCapture grava uma captura com status "gravando" e o arquivo
// capture-<id>.pcap, e a devolve com o ID do banco.
func (s *Store) CreateCapture(ctx context.Context, c capture.Capture) (capture.Capture, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return c, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		"INSERT INTO captures (started_ms, duration_s, port, status) VALUES (?, ?, ?, ?)",
		c.StartedAt.UnixMilli(), c.Duration, c.Port, capture.StatusRecording)
	if err != nil {
		return c, err
	}
	if c.ID, err = res.LastInsertId(); err != nil {
		return c, err
	}
	c.File = capture.FileName(c.ID)
	if _, err := tx.ExecContext(ctx, "UPDATE captures SET file = ? WHERE id = ?", c.File, c.ID); err != nil {
		return c, err
	}
	c.Status = capture.StatusRecording
	c.StartedAt = c.StartedAt.Truncate(time.Millisecond).UTC()
	return c, tx.Commit()
}

// FinishCapture grava o fim da captura com o status, os contadores e o erro.
func (s *Store) FinishCapture(ctx context.Context, c capture.Capture) error {
	var finished int64
	if c.FinishedAt != nil {
		finished = c.FinishedAt.UnixMilli()
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE captures SET status = ?, error = ?, finished_ms = ?, packets = ?, bytes = ?, dropped = ?
		 WHERE id = ?`,
		c.Status, c.Error, finished, c.Packets, c.Bytes, c.Dropped, c.ID)
	if err != nil {
		return err
	}
	return captureAffected(res)
}

// FailInterruptedCaptures marca como "falhou" as capturas que ficaram
// gravando quando o Omni foi encerrado.
func (s *Store) FailInterruptedCaptures(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE captures SET status = ?, error = ?, finished_ms = ? WHERE status = ?",
		capture.StatusFailed, "interrompida: o Omni foi encerrado durante a gravação", time.Now().UnixMilli(),
		capture.StatusRecording)
	return err
}

// ListCaptures devolve as capturas, das mais novas para as mais antigas.
func (s *Store) ListCaptures(ctx context.Context) ([]capture.Capture, error) {
	return s.queryCaptures(ctx, "SELECT "+captureColumns+" FROM captures ORDER BY started_ms DESC, id DESC")
}

// CapturesBefore devolve as capturas iniciadas antes de before que não estão gravando.
func (s *Store) CapturesBefore(ctx context.Context, before time.Time) ([]capture.Capture, error) {
	return s.queryCaptures(ctx,
		"SELECT "+captureColumns+" FROM captures WHERE started_ms < ? AND status != ? ORDER BY id",
		before.UnixMilli(), capture.StatusRecording)
}

// GetCapture devolve uma captura ou capture.ErrNotFound.
func (s *Store) GetCapture(ctx context.Context, id int64) (capture.Capture, error) {
	list, err := s.queryCaptures(ctx, "SELECT "+captureColumns+" FROM captures WHERE id = ?", id)
	if err != nil {
		return capture.Capture{}, err
	}
	if len(list) == 0 {
		return capture.Capture{}, capture.ErrNotFound
	}
	return list[0], nil
}

// DeleteCapture apaga a linha da captura; o arquivo é com quem chama.
func (s *Store) DeleteCapture(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM captures WHERE id = ?", id)
	if err != nil {
		return err
	}
	return captureAffected(res)
}

func captureAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return capture.ErrNotFound
	}
	return nil
}

func (s *Store) queryCaptures(ctx context.Context, query string, args ...any) ([]capture.Capture, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []capture.Capture{}
	for rows.Next() {
		var (
			c        capture.Capture
			started  int64
			finished sql.NullInt64
		)
		if err := rows.Scan(&c.ID, &c.File, &started, &finished, &c.Duration, &c.Port,
			&c.Packets, &c.Bytes, &c.Dropped, &c.Status, &c.Error); err != nil {
			return nil, err
		}
		c.StartedAt = time.UnixMilli(started).UTC()
		if finished.Valid && finished.Int64 > 0 {
			t := time.UnixMilli(finished.Int64).UTC()
			c.FinishedAt = &t
		}
		list = append(list, c)
	}
	return list, rows.Err()
}
