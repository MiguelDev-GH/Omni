// Package store guarda os produtos e pedidos da loja cobaia.
package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound    = errors.New("não encontrado")
	ErrOutOfStock  = errors.New("estoque insuficiente")
	ErrUnavailable = errors.New("banco indisponível")
)

type Product struct {
	ID         int64  `json:"id"`
	Name       string `json:"nome"`
	PriceCents int    `json:"preco_centavos"`
	Stock      int    `json:"estoque"`
}

type Order struct {
	ID        int64     `json:"id"`
	ProductID int64     `json:"produto_id"`
	Quantity  int       `json:"quantidade"`
	CreatedAt time.Time `json:"criado_em"`
}

// Store é o que a API da loja precisa do banco.
type Store interface {
	Ping(ctx context.Context) error
	ListProducts(ctx context.Context, limit, offset int) ([]Product, error)
	GetProduct(ctx context.Context, id int64) (Product, error)
	SearchProducts(ctx context.Context, q string) ([]Product, error)
	CreateOrder(ctx context.Context, productID int64, qty int) (Order, error)
	GetOrder(ctx context.Context, id int64) (Order, error)
	// Sleep segura uma consulta no banco por d, para gerar latência.
	Sleep(ctx context.Context, d time.Duration) error
	// Broken roda uma consulta que sempre falha no banco.
	Broken(ctx context.Context) error
}
