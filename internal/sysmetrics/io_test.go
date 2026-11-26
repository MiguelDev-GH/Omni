package sysmetrics

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceForPicksLongestMountpoint(t *testing.T) {
	parts := []disk.PartitionStat{
		{Device: "/dev/sda1", Mountpoint: "/"},
		{Device: "/dev/sdb1", Mountpoint: "/var"},
		{Device: "/dev/nvme0n1p2", Mountpoint: "/var/lib"},
	}
	assert.Equal(t, "nvme0n1p2", deviceFor("/var/lib/omni", parts))
	assert.Equal(t, "sdb1", deviceFor("/var/log", parts))
	assert.Equal(t, "sda1", deviceFor("/variavel", parts), "/var não contém /variavel")
	assert.Equal(t, "sda1", deviceFor("/home/u", parts))
	assert.Equal(t, "", deviceFor("/home/u", nil))
}

func TestIORates(t *testing.T) {
	prev := disk.IOCountersStat{ReadBytes: 1000, ReadCount: 10, WriteCount: 20, ReadTime: 100, WriteTime: 200, IoTime: 3000}
	cur := disk.IOCountersStat{
		ReadBytes: 1000 + 1<<20, WriteBytes: 4096 * 10,
		ReadCount: 60, WriteCount: 70,
		ReadTime: 300, WriteTime: 500,
		IoTime: 5000,
	}
	r := ioRates(prev, cur, 10*time.Second)
	assert.InDelta(t, 104857.6, r.ReadBps, 1e-6)
	assert.InDelta(t, 4096.0, r.WriteBps, 1e-6)
	assert.InDelta(t, 10.0, r.IOPS, 1e-9)
	assert.InDelta(t, 5.0, r.LatencyMS, 1e-9)
	assert.InDelta(t, 20.0, r.Busy, 1e-9)
}

func TestIORatesCounterReset(t *testing.T) {
	prev := disk.IOCountersStat{ReadBytes: 5000, ReadCount: 100, IoTime: 9000}
	cur := disk.IOCountersStat{ReadBytes: 10, ReadCount: 1, IoTime: 5}
	assert.Equal(t, IORates{}, ioRates(prev, cur, 5*time.Second))
}

func TestIORatesZeroInterval(t *testing.T) {
	c := disk.IOCountersStat{ReadBytes: 10, ReadCount: 1}
	assert.Equal(t, IORates{}, ioRates(disk.IOCountersStat{}, c, 0))
}

func TestIORatesZeroOps(t *testing.T) {
	c := disk.IOCountersStat{ReadBytes: 10, ReadCount: 1, ReadTime: 4, IoTime: 7}
	r := ioRates(c, c, 5*time.Second)
	assert.Equal(t, 0.0, r.LatencyMS)
	assert.Equal(t, IORates{}, r)
}

func TestIORatesBusyCappedAt100(t *testing.T) {
	r := ioRates(disk.IOCountersStat{}, disk.IOCountersStat{ReadCount: 1, IoTime: 6000}, 5*time.Second)
	assert.Equal(t, 100.0, r.Busy)
}

// TestSamplerFirstSampleHasNoIO: sem leitura anterior não há taxa.
func TestSamplerFirstSampleHasNoIO(t *testing.T) {
	s := NewSampler(t.TempDir())
	snap, err := s.Sample(context.Background())
	require.NoError(t, err)
	assert.False(t, snap.IO)
	assert.Zero(t, snap.IOPS)
}

// TestSamplerReadsRealDiskOnLinux confere que o dispositivo do diretório
// temporário é encontrado e que a segunda leitura traz o I/O.
func TestSamplerReadsRealDiskOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("contadores de disco por dispositivo só são testados no Linux")
	}
	s := NewSampler(t.TempDir())
	_, err := s.Sample(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, s.device)
	time.Sleep(50 * time.Millisecond)
	snap, err := s.Sample(context.Background())
	require.NoError(t, err)
	assert.True(t, snap.IO)
	assert.GreaterOrEqual(t, snap.DiskBusy, 0.0)
	assert.LessOrEqual(t, snap.DiskBusy, 100.0)
}
