// Comando loja sobe a API da loja cobaia no loopback.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MiguelDev-GH/omni/subjects/loja/internal/config"
	"github.com/MiguelDev-GH/omni/subjects/loja/internal/httpapi"
	"github.com/MiguelDev-GH/omni/subjects/loja/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("loja parou", "erro", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	// A API sobe antes do banco: o Poller do Omni precisa ver GET / respondendo.
	go func() {
		if err := store.MigrateWithRetry(ctx, db, 2*time.Second); err == nil {
			slog.Info("banco pronto")
		}
	}()

	srv := &http.Server{Addr: cfg.Addr, Handler: httpapi.New(db), ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	slog.Info("loja ouvindo em " + cfg.Addr)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
