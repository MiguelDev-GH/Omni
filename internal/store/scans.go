package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/MiguelDev-GH/omni/internal/findings"
)

// ErrScanNotFound indica uma varredura inexistente.
var ErrScanNotFound = errors.New("varredura não encontrada")

// Estados de uma varredura.
const (
	ScanQueued   = "fila"
	ScanRunning  = "rodando"
	ScanDone     = "concluida"
	ScanFailed   = "falhou"
	maxScansList = 200
)

// Scan é uma execução do Arsenal contra um alvo, com uma ou mais ferramentas.
type Scan struct {
	ID       int64    `json:"id"`
	TargetID int64    `json:"alvo_id"`
	Tools    []string `json:"ferramentas"`
	// Options são as opções de cada ferramenta (ferramenta → chave → valor).
	Options    map[string]map[string]string `json:"opcoes"`
	Status     string                       `json:"status"`
	Error      string                       `json:"erro,omitempty"`
	CreatedAt  time.Time                    `json:"criada_em"`
	StartedAt  *time.Time                   `json:"inicio,omitempty"`
	FinishedAt *time.Time                   `json:"fim,omitempty"`
	Counts     map[findings.Severity]int    `json:"achados"`
}

const scanColumns = "id, target_id, tools, options, status, error, created_ms, started_ms, finished_ms"

// CreateScan grava uma varredura na fila, com as opções de cada ferramenta.
func (s *Store) CreateScan(ctx context.Context, targetID int64, tools []string, opts map[string]map[string]string) (Scan, error) {
	b, err := json.Marshal(tools)
	if err != nil {
		return Scan{}, err
	}
	if opts == nil {
		opts = map[string]map[string]string{}
	}
	o, err := json.Marshal(opts)
	if err != nil {
		return Scan{}, err
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO scans (target_id, tools, options, status, created_ms) VALUES (?, ?, ?, ?, ?)",
		targetID, string(b), string(o), ScanQueued, now.UnixMilli())
	if err != nil {
		return Scan{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Scan{}, err
	}
	return Scan{ID: id, TargetID: targetID, Tools: tools, Options: opts, Status: ScanQueued, CreatedAt: now.Truncate(time.Millisecond), Counts: map[findings.Severity]int{}}, nil
}

// UpdateScanStatus muda o estado; "rodando" marca o início, e "concluida" ou
// "falhou" marcam o fim.
func (s *Store) UpdateScanStatus(ctx context.Context, id int64, status, errMsg string) error {
	now := time.Now().UnixMilli()
	var q string
	switch status {
	case ScanRunning:
		q = "UPDATE scans SET status = ?, error = ?, started_ms = ? WHERE id = ?"
	case ScanDone, ScanFailed:
		q = "UPDATE scans SET status = ?, error = ?, finished_ms = ? WHERE id = ?"
	default:
		return fmt.Errorf("estado de varredura inválido: %q", status)
	}
	res, err := s.db.ExecContext(ctx, q, status, errMsg, now, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrScanNotFound
	}
	return err
}

// FailInterruptedScans marca como falhas as varreduras que ficaram na fila
// ou rodando quando o Omni parou.
func (s *Store) FailInterruptedScans(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE scans SET status = ?, error = ?, finished_ms = ? WHERE status IN (?, ?)",
		ScanFailed, "interrompida: o Omni foi encerrado durante a varredura", time.Now().UnixMilli(), ScanQueued, ScanRunning)
	return err
}

// AddFindings grava os achados de uma varredura; achados com o mesmo
// Fingerprint na mesma varredura são gravados uma vez.
func (s *Store) AddFindings(ctx context.Context, scanID int64, list []findings.Finding) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, f := range list {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO findings (scan_id, tool, title, evidence, severity, port, fingerprint)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			scanID, f.Tool, f.Title, f.Evidence, string(f.Severity), f.Port, f.Fingerprint); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetScan devolve uma varredura com a contagem de achados por severidade.
func (s *Store) GetScan(ctx context.Context, id int64) (Scan, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+scanColumns+" FROM scans WHERE id = ?", id)
	sc, err := scanScan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Scan{}, ErrScanNotFound
	}
	if err != nil {
		return Scan{}, err
	}
	counts, err := s.severityCounts(ctx, []int64{id})
	if err != nil {
		return Scan{}, err
	}
	sc.Counts = counts[id]
	return sc, nil
}

// ListScans devolve as varreduras mais recentes primeiro, de um alvo ou de
// todos (targetID 0), no máximo 200.
func (s *Store) ListScans(ctx context.Context, targetID int64) ([]Scan, error) {
	q := "SELECT " + scanColumns + " FROM scans"
	args := []any{}
	if targetID > 0 {
		q += " WHERE target_id = ?"
		args = append(args, targetID)
	}
	q += " ORDER BY id DESC LIMIT ?"
	args = append(args, maxScansList)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []Scan{}
	var ids []int64
	for rows.Next() {
		sc, err := scanScan(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, sc)
		ids = append(ids, sc.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	counts, err := s.severityCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Counts = counts[list[i].ID]
	}
	return list, nil
}

// ScanFindings devolve os achados de uma varredura, dos mais graves aos menos graves.
func (s *Store) ScanFindings(ctx context.Context, scanID int64) ([]findings.Finding, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT tool, title, evidence, severity, port, fingerprint FROM findings WHERE scan_id = ?", scanID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []findings.Finding{}
	for rows.Next() {
		var (
			f   findings.Finding
			sev string
		)
		if err := rows.Scan(&f.Tool, &f.Title, &f.Evidence, &sev, &f.Port, &f.Fingerprint); err != nil {
			return nil, err
		}
		f.Severity = findings.Severity(sev)
		list = append(list, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	findings.Sort(list)
	return list, nil
}

// PreviousScan devolve a última varredura concluída do alvo, anterior a
// beforeID, que rodou a ferramenta tool; ok é false se não houver.
func (s *Store) PreviousScan(ctx context.Context, targetID, beforeID int64, tool string) (Scan, bool, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+scanColumns+" FROM scans WHERE target_id = ? AND id < ? AND status = ? ORDER BY id DESC",
		targetID, beforeID, ScanDone)
	if err != nil {
		return Scan{}, false, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		sc, err := scanScan(rows)
		if err != nil {
			return Scan{}, false, err
		}
		if slices.Contains(sc.Tools, tool) {
			return sc, true, nil
		}
	}
	return Scan{}, false, rows.Err()
}

func (s *Store) severityCounts(ctx context.Context, ids []int64) (map[int64]map[findings.Severity]int, error) {
	out := make(map[int64]map[findings.Severity]int, len(ids))
	for _, id := range ids {
		out[id] = map[findings.Severity]int{}
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT scan_id, severity, COUNT(*) FROM findings WHERE scan_id >= ? AND scan_id <= ? GROUP BY scan_id, severity",
		slices.Min(ids), slices.Max(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			id  int64
			sev string
			n   int
		)
		if err := rows.Scan(&id, &sev, &n); err != nil {
			return nil, err
		}
		if m, ok := out[id]; ok {
			m[findings.Severity(sev)] = n
		}
	}
	return out, rows.Err()
}

func scanScan(r scanner) (Scan, error) {
	var (
		sc                         Scan
		tools, options             string
		created, started, finished int64
	)
	if err := r.Scan(&sc.ID, &sc.TargetID, &tools, &options, &sc.Status, &sc.Error, &created, &started, &finished); err != nil {
		return Scan{}, err
	}
	if err := json.Unmarshal([]byte(tools), &sc.Tools); err != nil {
		return Scan{}, err
	}
	if err := json.Unmarshal([]byte(options), &sc.Options); err != nil {
		return Scan{}, err
	}
	sc.CreatedAt = time.UnixMilli(created).UTC()
	sc.StartedAt = optionalTime(started)
	sc.FinishedAt = optionalTime(finished)
	sc.Counts = map[findings.Severity]int{}
	return sc, nil
}

func optionalTime(ms int64) *time.Time {
	if ms == 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}
