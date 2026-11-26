package sysmetrics

import (
	"context"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
)

// IORates são as taxas de I/O do disco entre duas leituras dos contadores.
type IORates struct {
	ReadBps   float64 // bytes lidos por segundo
	WriteBps  float64 // bytes escritos por segundo
	IOPS      float64 // operações (leituras + escritas) por segundo
	LatencyMS float64 // tempo médio por operação
	Busy      float64 // % do intervalo com I/O em andamento, até 100
}

// deviceFor devolve o nome do dispositivo (como "sda1" ou "nvme0n1p2") da
// partição de mountpoint mais longo que contém p; "" quando nenhuma contém.
func deviceFor(p string, parts []disk.PartitionStat) string {
	best, device := -1, ""
	for _, part := range parts {
		mp := part.Mountpoint
		inside := p == mp || mp == "/" || strings.HasPrefix(p, strings.TrimSuffix(mp, "/")+"/")
		if inside && len(mp) > best {
			dev := part.Device
			// No LVM, /dev/mapper/vg-lv é um link para /dev/dm-0, o nome
			// usado nos contadores de disco.
			if real, err := filepath.EvalSymlinks(dev); err == nil {
				dev = real
			}
			best, device = len(mp), path.Base(filepath.ToSlash(dev))
		}
	}
	return device
}

// ioRates calcula as taxas entre duas leituras feitas com dt de intervalo.
// dt <= 0 ou um contador menor que o anterior (reinício, troca de
// dispositivo) devolve zeros.
func ioRates(prev, cur disk.IOCountersStat, dt time.Duration) IORates {
	if dt <= 0 ||
		cur.ReadBytes < prev.ReadBytes || cur.WriteBytes < prev.WriteBytes ||
		cur.ReadCount < prev.ReadCount || cur.WriteCount < prev.WriteCount ||
		cur.ReadTime < prev.ReadTime || cur.WriteTime < prev.WriteTime ||
		cur.IoTime < prev.IoTime {
		return IORates{}
	}
	secs := dt.Seconds()
	ops := float64(cur.ReadCount-prev.ReadCount) + float64(cur.WriteCount-prev.WriteCount)
	r := IORates{
		ReadBps:  float64(cur.ReadBytes-prev.ReadBytes) / secs,
		WriteBps: float64(cur.WriteBytes-prev.WriteBytes) / secs,
		IOPS:     ops / secs,
		Busy:     min(float64(cur.IoTime-prev.IoTime)/float64(dt.Milliseconds())*100, 100),
	}
	if ops > 0 {
		r.LatencyMS = (float64(cur.ReadTime-prev.ReadTime) + float64(cur.WriteTime-prev.WriteTime)) / ops
	}
	return r
}

// Sampler lê CPU, memória e disco como Sample e acrescenta o I/O do
// dispositivo que guarda diskPath, calculado entre uma leitura e a seguinte.
// Não é seguro para uso concorrente.
type Sampler struct {
	diskPath string
	device   string
	resolved bool
	prev     disk.IOCountersStat
	prevAt   time.Time
	hasPrev  bool
	warned   bool // o aviso de dispositivo sem contadores já saiu
}

func NewSampler(diskPath string) *Sampler {
	return &Sampler{diskPath: diskPath}
}

// Sample faz uma leitura. A primeira sai com IO false, porque ainda não há
// leitura anterior; também sai com IO false quando o dispositivo não é
// encontrado (como no Windows nativo).
func (s *Sampler) Sample(ctx context.Context) (Snapshot, error) {
	snap, err := Sample(ctx, s.diskPath)
	if err != nil {
		return snap, err
	}
	if !s.resolved {
		s.resolved = true
		s.device = s.findDevice(ctx)
		if s.device == "" {
			slog.Warn("disco: dispositivo não encontrado; I/O desligado", "caminho", s.diskPath)
		}
	}
	if s.device == "" {
		return snap, nil
	}
	counters, err := disk.IOCountersWithContext(ctx, s.device)
	cur, ok := counters[s.device]
	if err != nil || !ok {
		s.hasPrev = false
		if !s.warned {
			s.warned = true
			slog.Warn("disco: dispositivo sem contadores de I/O; I/O desligado", "dispositivo", s.device)
		}
		return snap, nil
	}
	if s.hasPrev {
		r := ioRates(s.prev, cur, snap.At.Sub(s.prevAt))
		snap.IO = true
		snap.ReadBps, snap.WriteBps, snap.IOPS = r.ReadBps, r.WriteBps, r.IOPS
		snap.DiskLatencyMS, snap.DiskBusy = r.LatencyMS, r.Busy
	}
	s.prev, s.prevAt, s.hasPrev = cur, snap.At, true
	return snap, nil
}

func (s *Sampler) findDevice(ctx context.Context) string {
	p, err := filepath.Abs(s.diskPath)
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		p = real
	}
	parts, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return ""
	}
	return deviceFor(filepath.ToSlash(p), parts)
}
