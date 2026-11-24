package store

import (
	"context"
	"time"

	"github.com/MiguelDev-GH/omni/internal/alerts"
)

// AddAlert grava uma mudança de nível e devolve o alerta com o ID do banco.
func (s *Store) AddAlert(ctx context.Context, a alerts.Alert) (alerts.Alert, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO alerts (time_ms, rule, service, name, subject, level, previous, value, threshold, unit, detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.At.UnixMilli(), a.Rule, a.Service, a.Name, a.Subject, string(a.Level), string(a.Previous),
		a.Value, a.Threshold, a.Unit, a.Detail)
	if err != nil {
		return a, err
	}
	a.ID, err = res.LastInsertId()
	return a, err
}

// AlertsBetween devolve até limit alertas com from <= hora < to, do mais novo
// para o mais antigo. Sem alertas, devolve slice vazio.
func (s *Store) AlertsBetween(ctx context.Context, from, to time.Time, limit int) ([]alerts.Alert, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, time_ms, rule, service, name, subject, level, previous, value, threshold, unit, detail
		 FROM alerts WHERE time_ms >= ? AND time_ms < ? ORDER BY time_ms DESC, id DESC LIMIT ?`,
		from.UnixMilli(), to.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	list := []alerts.Alert{}
	for rows.Next() {
		var (
			a           alerts.Alert
			ms          int64
			level, prev string
		)
		if err := rows.Scan(&a.ID, &ms, &a.Rule, &a.Service, &a.Name, &a.Subject, &level, &prev,
			&a.Value, &a.Threshold, &a.Unit, &a.Detail); err != nil {
			return nil, err
		}
		a.At = time.UnixMilli(ms).UTC()
		a.Level, a.Previous = alerts.Level(level), alerts.Level(prev)
		list = append(list, a)
	}
	return list, rows.Err()
}

// PruneAlerts apaga os alertas anteriores a before.
func (s *Store) PruneAlerts(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM alerts WHERE time_ms < ?", before.UnixMilli())
	return err
}
