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

const alertColumns = `id, time_ms, rule, service, name, subject, level, previous, value, threshold, unit, detail`

// AlertsBetween devolve até limit alertas com from <= hora < to, do mais novo
// para o mais antigo. Sem alertas, devolve slice vazio.
func (s *Store) AlertsBetween(ctx context.Context, from, to time.Time, limit int) ([]alerts.Alert, error) {
	return s.queryAlerts(ctx,
		`SELECT `+alertColumns+` FROM alerts WHERE time_ms >= ? AND time_ms < ?
		 ORDER BY time_ms DESC, id DESC LIMIT ?`,
		from.UnixMilli(), to.UnixMilli(), limit)
}

// WorstAlertsBetween devolve até limit alertas do intervalo, os mais graves
// primeiro: chegadas ao vermelho, depois ao amarelo, depois ao verde; no
// mesmo nível, do mais novo para o mais antigo.
func (s *Store) WorstAlertsBetween(ctx context.Context, from, to time.Time, limit int) ([]alerts.Alert, error) {
	return s.queryAlerts(ctx,
		`SELECT `+alertColumns+` FROM alerts WHERE time_ms >= ? AND time_ms < ?
		 ORDER BY CASE level WHEN 'vermelho' THEN 0 WHEN 'amarelo' THEN 1 ELSE 2 END, time_ms DESC, id DESC
		 LIMIT ?`,
		from.UnixMilli(), to.UnixMilli(), limit)
}

// AlertRuleSummary conta as mudanças de nível de uma regra no intervalo.
type AlertRuleSummary struct {
	Rule     string               `json:"regra"`
	Service  string               `json:"servico"`
	Name     string               `json:"nome"`
	Subjects int                  `json:"recursos"`
	Counts   map[alerts.Level]int `json:"mudancas"`
}

// AlertSummaryBetween conta, sem limite, as mudanças de nível de cada regra
// no intervalo, pelo nível de chegada, e quantos recursos diferentes mudaram.
// As regras com mais chegadas ao vermelho vêm primeiro.
func (s *Store) AlertSummaryBetween(ctx context.Context, from, to time.Time) ([]AlertRuleSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT rule, service, MAX(name), COUNT(DISTINCT subject),
		        SUM(level = 'verde'), SUM(level = 'amarelo'), SUM(level = 'vermelho')
		 FROM alerts WHERE time_ms >= ? AND time_ms < ?
		 GROUP BY rule, service
		 ORDER BY SUM(level = 'vermelho') DESC, SUM(level = 'amarelo') DESC, COUNT(*) DESC, rule`,
		from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	list := []AlertRuleSummary{}
	for rows.Next() {
		var (
			r                  AlertRuleSummary
			green, yellow, red int
		)
		if err := rows.Scan(&r.Rule, &r.Service, &r.Name, &r.Subjects, &green, &yellow, &red); err != nil {
			return nil, err
		}
		r.Counts = map[alerts.Level]int{alerts.Green: green, alerts.Yellow: yellow, alerts.Red: red}
		list = append(list, r)
	}
	return list, rows.Err()
}

func (s *Store) queryAlerts(ctx context.Context, query string, args ...any) ([]alerts.Alert, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
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
