package store

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cleanDB abre o banco de LOJA_TESTE_BANCO, apaga as tabelas e migra de novo.
func cleanDB(t *testing.T) *Postgres {
	t.Helper()
	url := os.Getenv("LOJA_TESTE_BANCO")
	if url == "" {
		t.Skip("defina LOJA_TESTE_BANCO")
	}
	ctx := context.Background()
	p, err := Open(ctx, url)
	require.NoError(t, err)
	t.Cleanup(p.Close)
	_, err = p.pool.Exec(ctx, "DROP TABLE IF EXISTS pedidos, produtos")
	require.NoError(t, err)
	require.NoError(t, p.Migrate(ctx))
	return p
}

func TestMigrateSeedsOnce(t *testing.T) {
	p := cleanDB(t)
	ctx := context.Background()
	require.NoError(t, p.Migrate(ctx))
	list, err := p.ListProducts(ctx, 100, 0)
	require.NoError(t, err)
	assert.Len(t, list, 50)
	assert.Equal(t, Product{ID: 1, Name: "Produto 01", PriceCents: 1137, Stock: 100}, list[0])
}

func TestListProductsPaginates(t *testing.T) {
	p := cleanDB(t)
	list, err := p.ListProducts(context.Background(), 5, 5)
	require.NoError(t, err)
	require.Len(t, list, 5)
	assert.Equal(t, int64(6), list[0].ID)
}

func TestGetProductNotFound(t *testing.T) {
	p := cleanDB(t)
	_, err := p.GetProduct(context.Background(), 999)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestSearchProducts(t *testing.T) {
	p := cleanDB(t)
	ctx := context.Background()
	list, err := p.SearchProducts(ctx, "o 0")
	require.NoError(t, err)
	assert.Len(t, list, 9)
	list, err = p.SearchProducts(ctx, "' OR 1=1 --")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestCreateOrder(t *testing.T) {
	p := cleanDB(t)
	ctx := context.Background()
	o, err := p.CreateOrder(ctx, 1, 3)
	require.NoError(t, err)
	assert.Equal(t, 3, o.Quantity)
	assert.Equal(t, int64(1), o.ProductID)
	prod, err := p.GetProduct(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, 97, prod.Stock)

	got, err := p.GetOrder(ctx, o.ID)
	require.NoError(t, err)
	assert.Equal(t, o.ID, got.ID)
	assert.Equal(t, o.Quantity, got.Quantity)
	assert.True(t, o.CreatedAt.Equal(got.CreatedAt))

	_, err = p.CreateOrder(ctx, 999, 1)
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = p.CreateOrder(ctx, 1, 101)
	assert.ErrorIs(t, err, ErrOutOfStock)
	_, err = p.GetOrder(ctx, 999)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestConcurrentOrdersNeverOversell(t *testing.T) {
	p := cleanDB(t)
	ctx := context.Background()
	_, err := p.pool.Exec(ctx, "UPDATE produtos SET estoque = 1 WHERE id = 2")
	require.NoError(t, err)

	var mu sync.Mutex
	ok, out := 0, 0
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.CreateOrder(ctx, 2, 1)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, ErrOutOfStock):
				out++
			default:
				t.Errorf("erro inesperado: %v", err)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, ok)
	assert.Equal(t, 19, out)
	prod, err := p.GetProduct(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, 0, prod.Stock)
}

func TestSleep(t *testing.T) {
	p := cleanDB(t)
	assert.NoError(t, p.Sleep(context.Background(), 0))
}

func TestBrokenReturnsPGError(t *testing.T) {
	p := cleanDB(t)
	err := p.Broken(context.Background())
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "42P01", pgErr.Code)
}

func TestUnavailable(t *testing.T) {
	ctx := context.Background()
	p, err := Open(ctx, "postgres://loja:x@127.0.0.1:1/loja?sslmode=disable")
	require.NoError(t, err)
	defer p.Close()
	assert.ErrorIs(t, p.Ping(ctx), ErrUnavailable)
	_, err = p.GetProduct(ctx, 1)
	assert.ErrorIs(t, err, ErrUnavailable)
}
