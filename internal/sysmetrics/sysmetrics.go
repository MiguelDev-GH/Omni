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

// Snapshot é uma leitura dos recursos do servidor, em percentual de uso.
type Snapshot struct {
	CPUPercent  float64   `json:"cpu"`
	MemPercent  float64   `json:"memoria"`
	DiskPercent float64   `json:"disco"`
	At          time.Time `json:"hora"`
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

// Run publica um Snapshot a cada interval até ctx ser cancelado. Falhas de
// coleta são registradas e o ciclo segue.
func Run(ctx context.Context, pub ws.Publisher, diskPath string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		snap, err := Sample(ctx, diskPath)
		switch {
		case err == nil:
			pub.Publish(ws.Event{Type: EventMetrics, Data: snap})
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
