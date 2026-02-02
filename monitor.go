package eitcache

import (
	"sync"
	"time"
)

// CacheMetrics stores cache metrics.
type CacheMetrics struct {
	HitCount        int64         `json:"hit_count"`
	MissCount       int64         `json:"miss_count"`
	EvictionCount   int64         `json:"eviction_count"`
	LastUpdate      time.Time     `json:"last_update"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
}

// Monitor tracks cache performance metrics.
type Monitor struct {
	mu       sync.RWMutex
	metrics  *CacheMetrics
	tracker  []time.Duration
	maxTrack int
}

// NewMonitor creates a cache monitor.
func NewMonitor() *Monitor {
	return &Monitor{
		metrics: &CacheMetrics{LastUpdate: time.Now()},
		tracker: make([]time.Duration, 0, 256),
		maxTrack: 1000,
	}
}

// RecordHit records a cache hit and its duration.
func (m *Monitor) RecordHit(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.HitCount++
	m.track(duration)
}

// RecordMiss records a cache miss and its duration.
func (m *Monitor) RecordMiss(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.MissCount++
	m.track(duration)
}

// RecordEviction adds eviction count.
func (m *Monitor) RecordEviction(count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.EvictionCount += count
}

// HitRatio returns cache hit ratio.
func (m *Monitor) HitRatio() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.metrics.HitCount + m.metrics.MissCount
	if total == 0 {
		return 0
	}
	return float64(m.metrics.HitCount) / float64(total)
}

// GetMetrics returns a snapshot of metrics.
func (m *Monitor) GetMetrics() CacheMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cp := *m.metrics
	return cp
}

// Reset clears metrics.
func (m *Monitor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics = &CacheMetrics{LastUpdate: time.Now()}
	m.tracker = make([]time.Duration, 0, m.maxTrack)
}

func (m *Monitor) track(duration time.Duration) {
	m.tracker = append(m.tracker, duration)
	if len(m.tracker) > m.maxTrack {
		m.tracker = m.tracker[1:]
	}

	var total time.Duration
	for _, d := range m.tracker {
		total += d
	}
	if len(m.tracker) > 0 {
		m.metrics.AvgResponseTime = total / time.Duration(len(m.tracker))
	}
	m.metrics.LastUpdate = time.Now()
}
