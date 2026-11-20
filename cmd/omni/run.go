package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/MiguelDev-GH/omni/internal/api"
	"github.com/MiguelDev-GH/omni/internal/config"
	"github.com/MiguelDev-GH/omni/internal/store"
)

const shutdownTimeout = 10 * time.Second

// run serve a API em ln até ctx ser cancelado; sempre fecha ln e o banco.
func run(ctx context.Context, cfg config.Config, ln net.Listener) error {
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		_ = ln.Close()
		return err
	}
	defer func() { _ = st.Close() }()

	srv := &http.Server{
		Handler:           api.NewServer(st, cfg.Token),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	slog.Info("omni no ar", "endereco", ln.Addr().String())

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
