// Package sysmetrics coleta CPU, memória e disco do servidor e os publica.
package sysmetrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/MiguelDev-GH/omni/internal/ws"
)

// DefaultInterval é o intervalo padrão entre coletas.
const DefaultInterval = 5 * time.Second

// EventMetrics é o tipo do evento publicado a cada coleta.
const EventMetrics = "METRICAS_SISTEMA"

const (
	// Retention é a retenção padrão das leituras no banco.
	Retention  = 24 * time.Hour
	pruneEvery = time.Minute
)

// Snapshot é uma leitura dos recursos do servidor: CPU, memória e disco em
// percentual de uso e, quando IO é true, o I/O do disco no intervalo.
type Snapshot struct {
	CPUPercent  float64 `json:"cpu"`
	MemPercent  float64 `json:"memoria"`
	DiskPercent float64 `json:"disco"`
	// IO é false quando o dispositivo do disco não foi encontrado ou ainda
	// não há leitura anterior; os campos de I/O ficam zerados.
	IO            bool      `json:"io"`
	ReadBps       float64   `json:"leitura_bps"`
	WriteBps      float64   `json:"escrita_bps"`
	IOPS          float64   `json:"iops"`
	DiskLatencyMS float64   `json:"latencia_disco_ms"` // tempo médio por operação
	DiskBusy      float64   `json:"disco_ocupado"`     // % do intervalo com I/O em andamento
	At            time.Time `json:"hora"`
}

// Sample lê CPU, memória e o disco que contém diskPath. O percentual de CPU
// vale desde a chamada anterior; a primeira chamada pode devolver 0.
func Sample(ctx context.Context, diskPath string) (Snapshot, error) {
	cpuPercents, err := cpu.PercentWithContext(ctx, 0, false)
	if err != nil {
		return Snapshot{}, fmt.Errorf("ler cpu: %w", err)
	}
	var cpuPercent float64
	if len(cpuPercents) > 0 {
		cpuPercent = cpuPercents[0]
	}

	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("ler memória: %w", err)
	}

	du, err := disk.UsageWithContext(ctx, diskPath)
	if err != nil {
		return Snapshot{}, fmt.Errorf("ler disco %q: %w", diskPath, err)
	}

	return Snapshot{
		CPUPercent:  cpuPercent,
		MemPercent:  vm.UsedPercent,
		DiskPercent: du.UsedPercent,
		At:          time.Now().UTC(),
	}, nil
}

// Sink guarda as leituras para o histórico.
type Sink interface {
	AddSystemSample(ctx context.Context, snap Snapshot) error
	// PruneSystem apaga as leituras anteriores a before.
	PruneSystem(ctx context.Context, before time.Time) error
}

// Run coleta um Snapshot a cada interval até ctx ser cancelado, grava no sink
// e publica. Falhas de coleta ou de gravação são registradas e o ciclo segue;
// uma falha ao gravar não impede a publicação. Uma vez por minuto, apaga as
// leituras mais antigas que retention.
func Run(ctx context.Context, pub ws.Publisher, sink Sink, diskPath string, interval, retention time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var lastPrune time.Time
	sampler := NewSampler(diskPath)
	for {
		snap, err := sampler.Sample(ctx)
		switch {
		case err == nil:
			if err := sink.AddSystemSample(ctx, snap); err != nil && ctx.Err() == nil {
				slog.Error("gravar métricas do sistema", "erro", err)
			}
			pub.Publish(ws.Event{Type: EventMetrics, Data: snap})
			if snap.At.Sub(lastPrune) >= pruneEvery {
				lastPrune = snap.At
				if err := sink.PruneSystem(ctx, snap.At.Add(-retention)); err != nil && ctx.Err() == nil {
					slog.Error("apagar métricas antigas do sistema", "erro", err)
				}
			}
		case !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded):
			slog.Error("coletar métricas do sistema", "erro", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
