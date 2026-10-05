package runtime_monitor

import (
	"testing"
	"time"
)

func TestAllocRateUsesTotalAllocNotMallocs(t *testing.T) {
	c := NewCollector(12)
	now := time.Now()
	first := &RuntimeMetrics{
		Timestamp:  now.Add(-60 * time.Second),
		TotalAlloc: 10 * 1024 * 1024,
		Mallocs:    1e12,
		HeapAlloc:  8 * 1024 * 1024,
		HeapInuse:  8 * 1024 * 1024,
	}
	last := &RuntimeMetrics{
		Timestamp:  now,
		TotalAlloc: 70 * 1024 * 1024,
		Mallocs:    2e12,
		HeapAlloc:  9 * 1024 * 1024,
		HeapInuse:  9 * 1024 * 1024,
	}

	c.mu.Lock()
	c.current = last
	c.history[0] = first
	c.history[1] = last
	c.historyIndex = 2
	c.mu.Unlock()

	stats := c.GetMemoryStats()
	if stats.AllocRate < 0.9 || stats.AllocRate > 1.1 {
		t.Fatalf("AllocRate = %v, want ~1 MB/s from TotalAlloc delta", stats.AllocRate)
	}
}

func TestPrometheusCounterAddsDeltaOnly(t *testing.T) {
	m := NewPrometheusMetrics("go", "runtime_test")
	c := NewCollector(4)

	first := &RuntimeMetrics{
		Timestamp:  time.Now(),
		Mallocs:    100,
		Frees:      40,
		TotalAlloc: 1024,
		NumCgoCall: 3,
	}
	c.mu.Lock()
	c.current = first
	c.history[0] = first
	c.historyIndex = 1
	c.mu.Unlock()

	m.Update(c)
	m.Update(c)

	second := &RuntimeMetrics{
		Timestamp:  time.Now(),
		Mallocs:    130,
		Frees:      55,
		TotalAlloc: 2048,
		NumCgoCall: 5,
		NumGC:      2,
	}
	c.mu.Lock()
	c.current = second
	c.history[1] = second
	c.historyIndex = 2
	c.mu.Unlock()
	m.Update(c)

	if m.lastMallocs != 130 || m.lastFrees != 55 || m.lastTotalAlloc != 2048 {
		t.Fatalf("last counters not updated: mallocs=%d frees=%d totalAlloc=%d", m.lastMallocs, m.lastFrees, m.lastTotalAlloc)
	}
}
