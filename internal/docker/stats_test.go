package docker

import (
	"testing"

	"github.com/docker/docker/api/types"
)

func TestComputeStats(t *testing.T) {
	raw := types.StatsJSON{}
	raw.CPUStats.CPUUsage.TotalUsage = 2_000_000_000
	raw.PreCPUStats.CPUUsage.TotalUsage = 1_000_000_000
	raw.CPUStats.SystemUsage = 20_000_000_000
	raw.PreCPUStats.SystemUsage = 10_000_000_000
	raw.CPUStats.OnlineCPUs = 4

	raw.MemoryStats.Usage = 200 * 1024 * 1024
	raw.MemoryStats.Limit = 1024 * 1024 * 1024
	raw.MemoryStats.Stats = map[string]uint64{"cache": 50 * 1024 * 1024}

	raw.Networks = map[string]types.NetworkStats{
		"eth0": {RxBytes: 100, TxBytes: 200},
		"eth1": {RxBytes: 10, TxBytes: 20},
	}

	got := computeStats(raw)

	// cpuDelta=1e9, systemDelta=1e10 -> (1e9/1e10)*4*100 = 40%
	if got.CPUPercent != 40 {
		t.Errorf("CPUPercent = %v, want 40", got.CPUPercent)
	}

	wantMem := uint64(200*1024*1024 - 50*1024*1024)
	if got.MemUsageBytes != wantMem {
		t.Errorf("MemUsageBytes = %d, want %d (cache subtracted)", got.MemUsageBytes, wantMem)
	}
	if got.MemLimitBytes != 1024*1024*1024 {
		t.Errorf("MemLimitBytes = %d, want %d", got.MemLimitBytes, 1024*1024*1024)
	}

	wantMemPercent := float64(wantMem) / float64(1024*1024*1024) * 100.0
	if got.MemPercent != wantMemPercent {
		t.Errorf("MemPercent = %v, want %v", got.MemPercent, wantMemPercent)
	}

	if got.NetRxBytes != 110 || got.NetTxBytes != 220 {
		t.Errorf("Net = rx:%d tx:%d, want rx:110 tx:220", got.NetRxBytes, got.NetTxBytes)
	}
}

func TestComputeStats_ZeroDeltasDontDivideByZero(t *testing.T) {
	got := computeStats(types.StatsJSON{})
	if got.CPUPercent != 0 {
		t.Errorf("CPUPercent = %v, want 0 for an all-zero sample", got.CPUPercent)
	}
	if got.MemPercent != 0 {
		t.Errorf("MemPercent = %v, want 0 when Limit is 0", got.MemPercent)
	}
}
