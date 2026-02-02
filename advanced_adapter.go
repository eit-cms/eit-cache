package eitcache

import (
	"context"
	"time"
)

// AdvancedRedisCacheAdapter wraps Redis adapter with monitoring.
type AdvancedRedisCacheAdapter struct {
	*RedisCacheAdapter
	monitor *Monitor
}

// NewAdvancedRedisCacheAdapter creates an advanced adapter.
func NewAdvancedRedisCacheAdapter(config *CacheConfig) (*AdvancedRedisCacheAdapter, error) {
	base, err := NewRedisCacheAdapter(config)
	if err != nil {
		return nil, err
	}
	return &AdvancedRedisCacheAdapter{
		RedisCacheAdapter: base,
		monitor:           NewMonitor(),
	}, nil
}

// Monitor returns adapter monitor.
func (a *AdvancedRedisCacheAdapter) Monitor() *Monitor {
	return a.monitor
}

// GetWithMonitoring retrieves cache and records metrics.
func (a *AdvancedRedisCacheAdapter) GetWithMonitoring(ctx context.Context, key string) ([]byte, error) {
	start := time.Now()
	data, err := a.RedisCacheAdapter.Get(ctx, key)
	duration := time.Since(start)
	if err == nil && data != nil {
		a.monitor.RecordHit(duration)
	} else {
		a.monitor.RecordMiss(duration)
	}
	return data, err
}

// SetWithMonitoring stores cache and records metrics.
func (a *AdvancedRedisCacheAdapter) SetWithMonitoring(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	err := a.RedisCacheAdapter.Set(ctx, key, value, ttl)
	duration := time.Since(start)
	if err == nil {
		a.monitor.RecordHit(duration)
	} else {
		a.monitor.RecordMiss(duration)
	}
	return err
}
