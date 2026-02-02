package eitcache

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

const (
	CacheTypeRedis  = "redis"
	CacheTypeMemory = "memory"
)

// CacheConfig configures cache manager and adapter.
type CacheConfig struct {
	Type       string
	Addr       string
	Password   string
	DB         int
	DefaultTTL time.Duration
	MaxRetries int
	PoolSize   int
	Prefix     string
}

// Manager orchestrates caching.
type Manager struct {
	adapter    Adapter
	defaultTTL time.Duration
	monitor    *Monitor
}

// NewManager creates a cache manager using CacheConfig.
func NewManager(config *CacheConfig) (*Manager, error) {
	if config == nil {
		config = &CacheConfig{Type: CacheTypeMemory}
	}

	var adapter Adapter
	var err error

	switch config.Type {
	case "", CacheTypeMemory:
		adapter = NewMemoryCacheAdapter(config.DefaultTTL)
	case CacheTypeRedis:
		adapter, err = NewRedisCacheAdapter(config)
	default:
		return nil, ErrInvalidType
	}

	if err != nil {
		return nil, err
	}

	return &Manager{
		adapter:    adapter,
		defaultTTL: config.DefaultTTL,
		monitor:    NewMonitor(),
	}, nil
}

// NewManagerWithAdapter creates a manager from an existing adapter.
func NewManagerWithAdapter(adapter Adapter, defaultTTL time.Duration) *Manager {
	return &Manager{
		adapter:    adapter,
		defaultTTL: defaultTTL,
		monitor:    NewMonitor(),
	}
}

// Adapter exposes the underlying adapter.
func (m *Manager) Adapter() Adapter {
	return m.adapter
}

// Monitor returns the cache monitor.
func (m *Manager) Monitor() *Monitor {
	return m.monitor
}

// Close closes the adapter.
func (m *Manager) Close() error {
	if m.adapter == nil {
		return nil
	}
	return m.adapter.Close()
}

// Set writes data to cache.
func (m *Manager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if m.adapter == nil {
		return errors.New("cache adapter is nil")
	}
	if ttl == 0 {
		ttl = m.defaultTTL
	}
	return m.adapter.Set(ctx, key, value, ttl)
}

// Get reads data from cache into dest. Returns hit status.
func (m *Manager) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	if m.adapter == nil {
		return false, errors.New("cache adapter is nil")
	}
	data, err := m.adapter.Get(ctx, key)
	if err != nil || data == nil {
		return false, err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, err
	}
	return true, nil
}

// Delete removes cached keys.
func (m *Manager) Delete(ctx context.Context, keys ...string) error {
	if m.adapter == nil {
		return errors.New("cache adapter is nil")
	}
	return m.adapter.Delete(ctx, keys...)
}

// DeletePattern removes cached keys by prefix pattern.
func (m *Manager) DeletePattern(ctx context.Context, pattern string) (int64, error) {
	if m.adapter == nil {
		return 0, errors.New("cache adapter is nil")
	}
	return m.adapter.DeletePattern(ctx, pattern)
}

// Exists checks if a key exists.
func (m *Manager) Exists(ctx context.Context, key string) (bool, error) {
	if m.adapter == nil {
		return false, errors.New("cache adapter is nil")
	}
	return m.adapter.Exists(ctx, key)
}

// Stats returns adapter stats.
func (m *Manager) Stats(ctx context.Context) (map[string]interface{}, error) {
	if m.adapter == nil {
		return nil, errors.New("cache adapter is nil")
	}
	return m.adapter.Stats(ctx)
}

// Ping checks adapter health.
func (m *Manager) Ping(ctx context.Context) error {
	if m.adapter == nil {
		return errors.New("cache adapter is nil")
	}
	return m.adapter.Ping(ctx)
}

// QueryOptions controls Query behavior.
type QueryOptions struct {
	TTL      time.Duration
	UseCache bool
	Ticket   *CacheTicket
}

// QueryOption mutates QueryOptions.
type QueryOption func(*QueryOptions)

// WithTTL sets cache TTL for Query.
func WithTTL(ttl time.Duration) QueryOption {
	return func(o *QueryOptions) {
		o.TTL = ttl
	}
}

// WithNoCache disables cache for Query.
func WithNoCache() QueryOption {
	return func(o *QueryOptions) {
		o.UseCache = false
	}
}

// WithTicket validates ticket before Query.
func WithTicket(ticket *CacheTicket) QueryOption {
	return func(o *QueryOptions) {
		o.Ticket = ticket
	}
}

// Query runs a cached query with generic result.
func Query[T any](ctx context.Context, manager *Manager, key string, queryFunc func() (T, error), opts ...QueryOption) (T, error) {
	var zero T
	if manager == nil {
		return zero, ErrManagerNil
	}
	if manager.adapter == nil {
		return zero, errors.New("cache adapter is nil")
	}

	options := &QueryOptions{
		TTL:      manager.defaultTTL,
		UseCache: true,
	}
	for _, opt := range opts {
		opt(options)
	}

	if options.Ticket != nil {
		if err := options.Ticket.Validate(); err != nil {
			return zero, err
		}
	}

	if options.UseCache {
		start := time.Now()
		data, err := manager.adapter.Get(ctx, key)
		elapsed := time.Since(start)
		if err == nil && data != nil {
			if manager.monitor != nil {
				manager.monitor.RecordHit(elapsed)
			}
			var cached T
			if err := json.Unmarshal(data, &cached); err == nil {
				return cached, nil
			}
		} else if manager.monitor != nil {
			manager.monitor.RecordMiss(elapsed)
		}
	}

	result, err := queryFunc()
	if err != nil {
		return zero, err
	}

	if options.UseCache {
		ttl := options.TTL
		if ttl == 0 {
			ttl = manager.defaultTTL
		}
		_ = manager.adapter.Set(ctx, key, result, ttl)
	}

	return result, nil
}
