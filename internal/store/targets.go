package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/MiguelDev-GH/omni/internal/target"
)

var ErrNotFound = errors.New("alvo não encontrado")

const targetColumns = "id, name, host, app_ports, db_port, domain, dns_port, smtp_port"

// CreateTarget ignora t.ID e devolve o alvo com o ID gerado.
func (s *Store) CreateTarget(ctx context.Context, t target.Target) (target.Target, error) {
	ports, err := json.Marshal(t.AppPorts)
	if err != nil {
		return target.Target{}, err
	}
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO targets (name, host, app_ports, db_port, domain, dns_port, smtp_port) VALUES (?, ?, ?, ?, ?, ?, ?)",
		t.Name, t.Host, string(ports), t.DBPort, t.Domain, t.DNSPort, t.SMTPPort)
	if err != nil {
		return target.Target{}, err
	}
	t.ID, err = res.LastInsertId()
	if err != nil {
		return target.Target{}, err
	}
	return t, nil
}

// ListTargets devolve os alvos ordenados por ID; sem alvos, devolve slice vazio.
func (s *Store) ListTargets(ctx context.Context) ([]target.Target, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+targetColumns+" FROM targets ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	list := []target.Target{}
	for rows.Next() {
		t, err := scanTarget(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (s *Store) GetTarget(ctx context.Context, id int64) (target.Target, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+targetColumns+" FROM targets WHERE id = ?", id)
	t, err := scanTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return target.Target{}, ErrNotFound
	}
	return t, err
}

func (s *Store) UpdateTarget(ctx context.Context, t target.Target) error {
	ports, err := json.Marshal(t.AppPorts)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		"UPDATE targets SET name = ?, host = ?, app_ports = ?, db_port = ?, domain = ?, dns_port = ?, smtp_port = ? WHERE id = ?",
		t.Name, t.Host, string(ports), t.DBPort, t.Domain, t.DNSPort, t.SMTPPort, t.ID)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

func (s *Store) DeleteTarget(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM targets WHERE id = ?", id)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

func requireAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface{ Scan(dest ...any) error }

func scanTarget(sc scanner) (target.Target, error) {
	var t target.Target
	var ports string
	if err := sc.Scan(&t.ID, &t.Name, &t.Host, &ports, &t.DBPort, &t.Domain, &t.DNSPort, &t.SMTPPort); err != nil {
		return target.Target{}, err
	}
	if err := json.Unmarshal([]byte(ports), &t.AppPorts); err != nil {
		return target.Target{}, err
	}
	return t, nil
}
