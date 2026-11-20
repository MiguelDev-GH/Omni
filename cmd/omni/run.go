package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/MiguelDev-GH/omni/internal/api"
	"github.com/MiguelDev-GH/omni/internal/config"
	"github.com/MiguelDev-GH/omni/internal/discovery"
	"github.com/MiguelDev-GH/omni/internal/poller"
	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/MiguelDev-GH/omni/internal/sysmetrics"
	"github.com/MiguelDev-GH/omni/internal/web"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

const (
	shutdownTimeout = 10 * time.Second
	clientBuffer    = 256
	ticketTTL       = 30 * time.Second
)

// run serve a API, o WebSocket, o poller e as métricas do sistema até ctx ser
// cancelado ou o servidor HTTP falhar. Sempre fecha ln e, só depois de todas as
// goroutines terminarem, o banco.
func run(ctx context.Context, cfg config.Config, ln net.Listener) error {
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		_ = ln.Close()
		return err
	}
	defer func() { _ = st.Close() }()

	hub := ws.NewHub(clientBuffer)
	tickets := ws.NewTickets(ticketTTL)

	srv := api.NewServer(st, cfg.Token)
	srv.Handle("POST /api/ws/ticket", ws.TicketHandler(tickets))
	srv.Handle("GET /api/alvos/{id}/historico", api.HistoryHandler(st))
	if tcp, ok := ln.Addr().(*net.TCPAddr); ok {
		srv.Handle("GET /api/descoberta", api.DiscoveryHandler(discovery.SystemListeners, tcp.Port))
	}
	srv.HandlePublic("GET /ws", ws.Handler(hub, tickets, cfg.AllowedOrigins))
	srv.HandleFrontend(web.Handler(web.Assets()))

	// Os defers rodam ao contrário: hub.Close (também quando Serve falha, pois
	// RegisterOnShutdown só roda no Shutdown), depois o fim das goroutines de
	// fundo e, por último, st.Close.
	bgCtx, stopBg := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() {
		stopBg()
		wg.Wait()
	}()
	defer hub.Close()

	wg.Add(2)
	go func() {
		defer wg.Done()
		poller.NewScheduler(st, hub, cfg.PollInterval).Run(bgCtx)
	}()
	go func() {
		defer wg.Done()
		sysmetrics.Run(bgCtx, hub, filepath.Dir(cfg.DBPath), sysmetrics.DefaultInterval)
	}()

	httpSrv := &http.Server{
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
	}
	httpSrv.RegisterOnShutdown(hub.Close)
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(ln) }()
	slog.Info("omni no ar", "endereco", ln.Addr().String())

	select {
	case err := <-serveErr:
		_ = httpSrv.Close()
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		_ = httpSrv.Close()
		return err
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
