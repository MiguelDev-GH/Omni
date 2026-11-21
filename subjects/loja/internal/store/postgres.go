package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS produtos (
	id serial PRIMARY KEY,
	nome text NOT NULL,
	preco_centavos integer NOT NULL,
	estoque integer NOT NULL CHECK (estoque >= 0)
);
CREATE TABLE IF NOT EXISTS pedidos (
	id serial PRIMARY KEY,
	produto_id integer NOT NULL REFERENCES produtos (id),
	quantidade integer NOT NULL,
	criado_em timestamptz NOT NULL DEFAULT now()
);`

// seed insere 50 produtos só quando a tabela está vazia.
const seed = `
INSERT INTO produtos (nome, preco_centavos, estoque)
SELECT 'Produto ' || lpad(i::text, 2, '0'), 1000 + i * 137, 100
FROM generate_series(1, 50) AS i
WHERE NOT EXISTS (SELECT 1 FROM produtos)`

// Postgres implementa Store com um pool do pgx.
type Postgres struct {
	pool      *pgxpool.Pool
	migrateMu sync.Mutex
}

var _ Store = (*Postgres)(nil)

// Open cria o pool sem conectar: o banco pode estar fora do ar.
func Open(ctx context.Context, url string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	cfg.ConnConfig.ConnectTimeout = 2 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() { p.pool.Close() }

// Migrate cria as tabelas e os dados iniciais.
func (p *Postgres) Migrate(ctx context.Context) error {
	p.migrateMu.Lock()
	defer p.migrateMu.Unlock()
	if _, err := p.pool.Exec(ctx, schema); err != nil {
		return classify(err)
	}
	_, err := p.pool.Exec(ctx, seed)
	return classify(err)
}

// Ping confere a conexão e se as tabelas existem; sem elas, migra de novo.
func (p *Postgres) Ping(ctx context.Context) error {
	var ok bool
	if err := p.pool.QueryRow(ctx, "SELECT to_regclass('produtos') IS NOT NULL AND to_regclass('pedidos') IS NOT NULL").Scan(&ok); err != nil {
		return classify(err)
	}
	if !ok {
		return p.remigrate(ctx)
	}
	return nil
}

// check classifica err e, se as tabelas da loja sumiram (o volume do banco foi
// recriado com a loja rodando), migra de novo e devolve ErrUnavailable.
func (p *Postgres) check(ctx context.Context, err error) error {
	err = classify(err)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return p.remigrate(ctx)
	}
	return err
}

func (p *Postgres) remigrate(ctx context.Context) error {
	if err := p.Migrate(ctx); err != nil {
		slog.Warn("migração do banco falhou", "erro", err)
	}
	return fmt.Errorf("%w: tabelas ausentes, migração refeita", ErrUnavailable)
}

func (p *Postgres) ListProducts(ctx context.Context, limit, offset int) ([]Product, error) {
	rows, err := p.pool.Query(ctx,
		"SELECT id, nome, preco_centavos, estoque FROM produtos ORDER BY id LIMIT $1 OFFSET $2", limit, offset)
	return p.collectProducts(ctx, rows, err)
}

func (p *Postgres) GetProduct(ctx context.Context, id int64) (Product, error) {
	var pr Product
	err := p.pool.QueryRow(ctx, "SELECT id, nome, preco_centavos, estoque FROM produtos WHERE id = $1", id).
		Scan(&pr.ID, &pr.Name, &pr.PriceCents, &pr.Stock)
	return pr, p.check(ctx, err)
}

// SearchProducts usa o protocolo simples, para que o sniffer do Omni veja uma
// consulta com o literal embutido.
func (p *Postgres) SearchProducts(ctx context.Context, q string) ([]Product, error) {
	rows, err := p.pool.Query(ctx,
		"SELECT id, nome, preco_centavos, estoque FROM produtos WHERE nome ILIKE '%' || $1 || '%' ORDER BY id LIMIT 20",
		pgx.QueryExecModeSimpleProtocol, q)
	return p.collectProducts(ctx, rows, err)
}

func (p *Postgres) CreateOrder(ctx context.Context, productID int64, qty int) (Order, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return Order{}, p.check(ctx, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, "UPDATE produtos SET estoque = estoque - $2 WHERE id = $1 AND estoque >= $2", productID, qty)
	if err != nil {
		return Order{}, p.check(ctx, err)
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM produtos WHERE id = $1)", productID).Scan(&exists); err != nil {
			return Order{}, p.check(ctx, err)
		}
		if !exists {
			return Order{}, ErrNotFound
		}
		return Order{}, ErrOutOfStock
	}

	o := Order{ProductID: productID, Quantity: qty}
	err = tx.QueryRow(ctx, "INSERT INTO pedidos (produto_id, quantidade) VALUES ($1, $2) RETURNING id, criado_em",
		productID, qty).Scan(&o.ID, &o.CreatedAt)
	if err != nil {
		return Order{}, p.check(ctx, err)
	}
	return o, p.check(ctx, tx.Commit(ctx))
}

func (p *Postgres) GetOrder(ctx context.Context, id int64) (Order, error) {
	var o Order
	err := p.pool.QueryRow(ctx, "SELECT id, produto_id, quantidade, criado_em FROM pedidos WHERE id = $1", id).
		Scan(&o.ID, &o.ProductID, &o.Quantity, &o.CreatedAt)
	return o, p.check(ctx, err)
}

func (p *Postgres) Sleep(ctx context.Context, d time.Duration) error {
	_, err := p.pool.Exec(ctx, "SELECT pg_sleep($1)", d.Seconds())
	return p.check(ctx, err)
}

func (p *Postgres) Broken(ctx context.Context) error {
	_, err := p.pool.Exec(ctx, "SELECT * FROM tabela_inexistente")
	return classify(err)
}

func (p *Postgres) collectProducts(ctx context.Context, rows pgx.Rows, err error) ([]Product, error) {
	if err != nil {
		return nil, p.check(ctx, err)
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Product, error) {
		var pr Product
		err := r.Scan(&pr.ID, &pr.Name, &pr.PriceCents, &pr.Stock)
		return pr, err
	})
	return list, p.check(ctx, err)
}

// classify traduz os erros do pgx para os erros sentinela do pacote.
func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// Classe 08 (conexão) e 57P (servidor desligando) indicam banco fora do ar.
		if len(pgErr.Code) == 5 && (pgErr.Code[:2] == "08" || pgErr.Code[:3] == "57P") {
			return fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		return err
	}
	var connErr *pgconn.ConnectError
	var netErr net.Error
	if errors.As(err, &connErr) || errors.As(err, &netErr) || pgconn.SafeToRetry(err) {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return err
}

// MigrateWithRetry tenta Migrate a cada intervalo até dar certo ou ctx acabar,
// para que a loja suba mesmo com o banco ainda fora do ar.
func MigrateWithRetry(ctx context.Context, m interface{ Migrate(context.Context) error }, every time.Duration) error {
	for {
		err := m.Migrate(ctx)
		if err == nil {
			return nil
		}
		slog.Warn("migração do banco falhou; tentando de novo", "erro", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(every):
		}
	}
}
