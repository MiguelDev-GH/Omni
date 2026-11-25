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

	"github.com/MiguelDev-GH/omni/internal/alerts"
	"github.com/MiguelDev-GH/omni/internal/api"
	"github.com/MiguelDev-GH/omni/internal/config"
	"github.com/MiguelDev-GH/omni/internal/discovery"
	"github.com/MiguelDev-GH/omni/internal/orchestrator"
	"github.com/MiguelDev-GH/omni/internal/poller"
	"github.com/MiguelDev-GH/omni/internal/sniffer"
	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/MiguelDev-GH/omni/internal/sysmetrics"
	"github.com/MiguelDev-GH/omni/internal/web"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

const (
	shutdownTimeout = 10 * time.Second
	clientBuffer    = 256
	ticketTTL       = 30 * time.Second
	// dockerEvery acompanha o ritmo do ESTADO_MOTORES do sniffer, para uma
	// aba recém-aberta saber logo o estado do Docker.
	dockerEvery = 5 * time.Second
)

// openCapture abre a captura ao vivo; os testes a trocam para não depender
// das permissões da máquina.
var openCapture = sniffer.OpenLive

// newRunner cria o executor dos containers do Arsenal; os testes o trocam
// para não depender do Docker da máquina.
var newRunner = func() (orchestrator.Runner, error) { return orchestrator.NewDockerRunner() }

// run serve a API, o WebSocket, o poller, o sniffer, o Arsenal e as métricas do sistema até ctx ser
// cancelado ou o servidor HTTP falhar. Sempre fecha ln e, só depois de todas as
// goroutines terminarem, o banco.
func run(ctx context.Context, cfg config.Config, ln net.Listener) error {
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		_ = ln.Close()
		return err
	}
	defer func() { _ = st.Close() }()

	if err := st.FailInterruptedScans(ctx); err != nil {
		slog.Error("arsenal: marcar varreduras interrompidas", "erro", err)
	}
	runner, err := newRunner()
	if err != nil {
		_ = ln.Close()
		return err
	}

	hub := ws.NewHub(clientBuffer)
	tickets := ws.NewTickets(ticketTTL)

	// Os motores publicam por pub: cada evento vai ao hub e uma cópia vai ao
	// observador de alertas, que avalia as regras e avisa as mudanças de nível.
	engine := alerts.NewEngine(alerts.DefaultRules())
	watcher := alerts.NewWatcher(engine, st)
	pub := alertTap{next: hub, feed: watcher.Feed}
	engine.OnAlert(func(a alerts.Alert) { hub.Publish(ws.Event{Type: alerts.EventAlert, Data: a}) })
	// Cada mudança de nível vai também para o banco, que alimenta os relatórios.
	engine.OnAlert(func(a alerts.Alert) {
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := st.AddAlert(saveCtx, a); err != nil {
			slog.Error("alertas: gravar no banco", "erro", err)
		}
	})
	var mailer *alerts.Mailer
	if cfg.Mail.Enabled() {
		mc := cfg.Mail
		mc.PanelURL = "http://" + ln.Addr().String() + "/alertas"
		mailer = alerts.NewMailer(mc)
		engine.OnAlert(mailer.Notify)
		slog.Info("alertas por e-mail ativos", "servidor", mc.Host, "porta", mc.Port, "destinatarios", len(mc.To))
	} else {
		slog.Info("alertas por e-mail desligados; defina OMNI_SMTP_HOST e OMNI_ALERTA_EMAILS para ligar")
	}

	srv := api.NewServer(st, cfg.Token)
	srv.Handle("POST /api/ws/ticket", ws.TicketHandler(tickets))
	srv.Handle("GET /api/alvos/{id}/historico", api.HistoryHandler(st))
	srv.Handle("GET /api/trafego", api.TrafficHandler(st))
	srv.Handle("GET /api/sistema", api.SystemHandler(st))
	srv.Handle("GET /api/requisicoes", api.RequestsHandler(st))
	srv.Handle("GET /api/requisicoes/rotas", api.RoutesHandler(st))
	srv.Handle("GET /api/consultas", api.QueriesHandler(st))
	srv.Handle("GET /api/consultas/lentas", api.SlowQueriesHandler(st))
	srv.Handle("GET /api/alertas/historico", api.AlertHistoryHandler(st))
	srv.Handle("GET /api/alvos/{id}/relatorio", api.ReportHandler(st))
	srv.Handle("GET /api/alertas", alerts.Handler(engine, alerts.EmailInfo{Enabled: cfg.Mail.Enabled(), Recipients: len(cfg.Mail.To)}))
	if tcp, ok := ln.Addr().(*net.TCPAddr); ok {
		srv.Handle("GET /api/descoberta", api.DiscoveryHandler(discovery.SystemListeners, tcp.Port))
	}
	// Os defers rodam ao contrário: hub.Close (também quando Serve falha, pois
	// RegisterOnShutdown só roda no Shutdown), depois o fim das goroutines de
	// fundo e das varreduras e, por último, st.Close.
	bgCtx, stopBg := context.WithCancel(ctx)
	var wg sync.WaitGroup
	orch := orchestrator.New(bgCtx, runner, st, hub)
	defer func() {
		stopBg()
		wg.Wait()
		orch.Wait()
	}()
	defer hub.Close()

	srv.Handle("GET /api/ferramentas", api.ToolsHandler())
	srv.Handle("POST /api/varreduras", api.StartScanHandler(orch))
	srv.Handle("GET /api/varreduras", api.ListScansHandler(st))
	srv.Handle("GET /api/varreduras/{id}", api.GetScanHandler(st))
	srv.Handle("GET /api/varreduras/{id}/achados", api.ScanFindingsHandler(st))
	srv.Handle("GET /api/varreduras/{id}/comparacao", api.CompareScanHandler(st, st))
	srv.Handle("GET /api/varreduras/{id}/relatorio/{ferramenta}", api.ScanReportHandler(st))
	srv.HandlePublic("GET /ws", ws.Handler(hub, tickets, cfg.AllowedOrigins))
	srv.HandleFrontend(web.Handler(web.Assets()))

	sec := startSecurity(bgCtx, &wg, engine, cfg)
	wg.Add(5)
	go func() {
		defer wg.Done()
		watcher.Run(bgCtx)
	}()
	if mailer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mailer.Run(bgCtx)
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		pruneAlerts(bgCtx, st, cfg.CheckRetention)
	}()
	go func() {
		defer wg.Done()
		poller.NewScheduler(st, pub, cfg.PollInterval).WithRetention(cfg.CheckRetention).Run(bgCtx)
	}()
	go func() {
		defer wg.Done()
		open := func(bpf string) (sniffer.Source, error) { return openCapture(cfg.Interface, bpf) }
		eng := sniffer.NewEngine(open, st, pub).WithRetention(cfg.CheckRetention)
		eng.OnHTTP(sec.observeHTTP)
		_ = eng.Run(bgCtx)
	}()
	go func() {
		defer wg.Done()
		orch.MonitorDocker(bgCtx, dockerEvery)
	}()
	go func() {
		defer wg.Done()
		sysmetrics.Run(bgCtx, pub, st, filepath.Dir(cfg.DBPath), sysmetrics.DefaultInterval, cfg.CheckRetention)
	}()

	httpSrv := &http.Server{
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
	}
	httpSrv.RegisterOnShutdown(hub.Close)
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(ln) }()
	slog.Info("omni no ar", "endereco", ln.Addr().String(), "painel", "http://"+ln.Addr().String())

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
