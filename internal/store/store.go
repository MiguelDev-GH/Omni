// Package store guarda os dados do Omni no SQLite.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Store struct {
	db *sql.DB
}

// Open abre o banco em path, aplica os PRAGMAs e as migrações pendentes.
func Open(ctx context.Context, path string) (*Store, error) {
	s, err := open(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("abrir banco %s: %w", path, err)
	}
	return s, nil
}

func open(ctx context.Context, path string) (*Store, error) {
	// O caminho é escapado para que '?', '#' e '%' façam parte do nome do arquivo.
	dsn := "file:" + (&url.URL{Path: path}).EscapedPath() +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := checkWritable(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// checkWritable falha já na abertura se o arquivo existir sem permissão de escrita.
func checkWritable(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version))
	return err
}

func (s *Store) Close() error { return s.db.Close() }

type migration struct {
	version int
	name    string
}

func migrate(ctx context.Context, db *sql.DB) error {
	var current int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	pending, err := pendingMigrations(current)
	if err != nil {
		return err
	}
	for _, m := range pending {
		if err := apply(ctx, db, m); err != nil {
			return fmt.Errorf("migração %s: %w", m.name, err)
		}
	}
	return nil
}

func pendingMigrations(current int) ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, e := range entries {
		prefix, _, ok := strings.Cut(e.Name(), "_")
		n, err := strconv.Atoi(prefix)
		if !ok || err != nil {
			return nil, fmt.Errorf("nome de migração inválido: %s", e.Name())
		}
		if n > current {
			out = append(out, migration{version: n, name: e.Name()})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func apply(ctx context.Context, db *sql.DB, m migration) error {
	body, err := migrationsFS.ReadFile("migrations/" + m.name)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.version)); err != nil {
		return err
	}
	return tx.Commit()
}
