package main

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/MiguelDev-GH/omni/internal/alerts"
	"github.com/MiguelDev-GH/omni/internal/config"
	"github.com/MiguelDev-GH/omni/internal/security"
	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
)

// integrityFile guarda a linha de base dos arquivos monitorados, ao lado do
// banco, para detectar alterações feitas com o Omni desligado.
const integrityFile = "integridade.json"

// securityDetectors liga os detectores de segurança ao motor de alertas.
type securityDetectors struct {
	traffic *security.Traffic
}

// startSecurity inicia os detectores de ataque no tráfego e o monitor de
// integridade dos arquivos de configuração.
func startSecurity(ctx context.Context, wg *sync.WaitGroup, engine *alerts.Engine, cfg config.Config) *securityDetectors {
	s := &securityDetectors{traffic: security.NewTraffic(engine)}
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.traffic.Run(ctx)
	}()
	if len(cfg.WatchedFiles) > 0 {
		files := security.NewFiles(engine, cfg.WatchedFiles, filepath.Join(filepath.Dir(cfg.DBPath), integrityFile))
		slog.Info("monitorando a integridade de arquivos de configuração", "arquivos", cfg.WatchedFiles)
		wg.Add(1)
		go func() {
			defer wg.Done()
			files.Run(ctx)
		}()
	} else {
		slog.Info("integridade de arquivos desligada; defina OMNI_ARQUIVOS_MONITORADOS para ligar")
	}
	return s
}

// observeHTTP entrega cada requisição decodificada pelo sniffer ao
// detector de ataques, menos o tráfego gerado pelo próprio Omni.
func (s *securityDetectors) observeHTTP(ex l7.HTTPExchange) {
	if ex.Internal {
		return
	}
	s.traffic.Observe(security.Request{IP: ex.OriginIP, User: ex.User, Status: ex.Status, At: ex.At})
}
