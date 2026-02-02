package eitcache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Adapter defines a cache backend.
type Adapter interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	DeletePattern(ctx context.Context, pattern string) (int64, error)
	Exists(ctx context.Context, key string) (bool, error)
	Incr(ctx context.Context, key string) (int64, error)
	Decr(ctx context.Context, key string) (int64, error)
	Stats(ctx context.Context) (map[string]interface{}, error)
	Ping(ctx context.Context) error
	Close() error
}

// RedisCacheAdapter implements Adapter with Redis.
type RedisCacheAdapter struct {
	client *redis.Client
	config *CacheConfig
	prefix string
}

// NewRedisCacheAdapter creates a Redis adapter.
func NewRedisCacheAdapter(config *CacheConfig) (*RedisCacheAdapter, error) {
	if config == nil {
		return nil, errors.New("redis cache config is nil")
	}

	addr := config.Addr
	if addr == "" {
		addr = "localhost:6379"
	}

	poolSize := config.PoolSize
	if poolSize <= 0 {
		poolSize = 10
	}

	client := redis.NewClient(&redis.Options{
		Addr:       addr,
		Password:   config.Password,
		DB:         config.DB,
		MaxRetries: config.MaxRetries,
		PoolSize:   poolSize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	prefix := config.Prefix
	if prefix == "" {
		prefix = "eit:cache:"
	}

	return &RedisCacheAdapter{
		client: client,
		config: config,
		prefix: prefix,
	}, nil
}

// Set stores a value.
func (r *RedisCacheAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal value failed: %w", err)
	}

	if ttl == 0 {
		ttl = r.config.DefaultTTL
	}

	return r.client.Set(ctx, r.prefix+key, payload, ttl).Err()
}

// Get retrieves cached bytes.
func (r *RedisCacheAdapter) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := r.client.Get(ctx, r.prefix+key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return data, err
}

// Delete deletes keys.
func (r *RedisCacheAdapter) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	fullKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		fullKeys = append(fullKeys, r.prefix+k)
	}
	return r.client.Del(ctx, fullKeys...).Err()
}

// DeletePattern deletes keys by prefix pattern.
func (r *RedisCacheAdapter) DeletePattern(ctx context.Context, pattern string) (int64, error) {
	if pattern == "" {
		return 0, nil
	}
	fullPattern := r.prefix + pattern
	if !strings.Contains(fullPattern, "*") {
		fullPattern += "*"
	}

	iter := r.client.Scan(ctx, 0, fullPattern, 200).Iterator()
	var count int64
	for iter.Next(ctx) {
		if err := r.client.Del(ctx, iter.Val()).Err(); err != nil {
			return count, err
		}
		count++
	}
	return count, iter.Err()
}

// Exists checks if a key exists.
func (r *RedisCacheAdapter) Exists(ctx context.Context, key string) (bool, error) {
	val, err := r.client.Exists(ctx, r.prefix+key).Result()
	return val > 0, err
}

// Incr increments a counter.
func (r *RedisCacheAdapter) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, r.prefix+key).Result()
}

// Decr decrements a counter.
func (r *RedisCacheAdapter) Decr(ctx context.Context, key string) (int64, error) {
	return r.client.Decr(ctx, r.prefix+key).Result()
}

// Stats returns redis stats.
func (r *RedisCacheAdapter) Stats(ctx context.Context) (map[string]interface{}, error) {
	info, err := r.client.Info(ctx, "memory").Result()
	if err != nil {
		return nil, err
	}
	count, err := r.client.DBSize(ctx).Result()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"db_size":    count,
		"redis_info": info,
	}, nil
}

// Ping checks redis health.
func (r *RedisCacheAdapter) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes redis connection.
func (r *RedisCacheAdapter) Close() error {
	return r.client.Close()
}

type memoryEntry struct {
	data     []byte
	expireAt time.Time
}

// MemoryCacheAdapter implements Adapter with in-memory map.
type MemoryCacheAdapter struct {
	mu         sync.RWMutex
	cache      map[string]*memoryEntry
	defaultTTL time.Duration
}

// NewMemoryCacheAdapter creates a memory adapter.
func NewMemoryCacheAdapter(defaultTTL time.Duration) *MemoryCacheAdapter {
	return &MemoryCacheAdapter{
		cache:      make(map[string]*memoryEntry),
		defaultTTL: defaultTTL,
	}
}

// Set stores a value in memory.
func (m *MemoryCacheAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	_ = ctx
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal value failed: %w", err)
	}

	if ttl == 0 {
		ttl = m.defaultTTL
	}

	var expireAt time.Time
	if ttl > 0 {
		expireAt = time.Now().Add(ttl)
	}

	m.mu.Lock()
	m.cache[key] = &memoryEntry{data: payload, expireAt: expireAt}
	m.mu.Unlock()
	return nil
}

// Get retrieves cached bytes.
func (m *MemoryCacheAdapter) Get(ctx context.Context, key string) ([]byte, error) {
	_ = ctx
	m.mu.RLock()
	entry, exists := m.cache[key]
	m.mu.RUnlock()
	if !exists {
		return nil, nil
	}

	if !entry.expireAt.IsZero() && time.Now().After(entry.expireAt) {
		m.mu.Lock()
		delete(m.cache, key)
		m.mu.Unlock()
		return nil, nil
	}

	return entry.data, nil
}

// Delete removes keys.
func (m *MemoryCacheAdapter) Delete(ctx context.Context, keys ...string) error {
	_ = ctx
	m.mu.Lock()
	for _, k := range keys {
		delete(m.cache, k)
	}
	m.mu.Unlock()
	return nil
}

// DeletePattern deletes keys with a prefix pattern.
func (m *MemoryCacheAdapter) DeletePattern(ctx context.Context, pattern string) (int64, error) {
	_ = ctx
	if pattern == "" {
		return 0, nil
	}
	prefix := strings.TrimSuffix(pattern, "*")
	var count int64
	m.mu.Lock()
	for k := range m.cache {
		if strings.HasPrefix(k, prefix) {
			delete(m.cache, k)
			count++
		}
	}
	m.mu.Unlock()
	return count, nil
}

// Exists checks if a key exists.
func (m *MemoryCacheAdapter) Exists(ctx context.Context, key string) (bool, error) {
	_ = ctx
	m.mu.RLock()
	entry, exists := m.cache[key]
	m.mu.RUnlock()
	if !exists {
		return false, nil
	}
	if !entry.expireAt.IsZero() && time.Now().After(entry.expireAt) {
		return false, nil
	}
	return true, nil
}

// Incr increments a counter.
func (m *MemoryCacheAdapter) Incr(ctx context.Context, key string) (int64, error) {
	return m.addDelta(ctx, key, 1)
}

// Decr decrements a counter.
func (m *MemoryCacheAdapter) Decr(ctx context.Context, key string) (int64, error) {
	return m.addDelta(ctx, key, -1)
}

func (m *MemoryCacheAdapter) addDelta(ctx context.Context, key string, delta int64) (int64, error) {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.cache[key]
	if exists && !entry.expireAt.IsZero() && time.Now().After(entry.expireAt) {
		delete(m.cache, key)
		exists = false
	}

	var current int64
	if exists {
		_ = json.Unmarshal(entry.data, &current)
	}
	current += delta

	payload, err := json.Marshal(current)
	if err != nil {
		return 0, err
	}

	expireAt := time.Time{}
	if exists {
		expireAt = entry.expireAt
	}
	m.cache[key] = &memoryEntry{data: payload, expireAt: expireAt}
	return current, nil
}

// Stats returns memory stats.
func (m *MemoryCacheAdapter) Stats(ctx context.Context) (map[string]interface{}, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := len(m.cache)
	expired := 0
	now := time.Now()
	for _, entry := range m.cache {
		if !entry.expireAt.IsZero() && now.After(entry.expireAt) {
			expired++
		}
	}

	return map[string]interface{}{
		"total_items":   total,
		"expired_items": expired,
		"active_items":  total - expired,
	}, nil
}

// Ping checks memory adapter health.
func (m *MemoryCacheAdapter) Ping(ctx context.Context) error {
	_ = ctx
	return nil
}

// Close closes memory adapter.
func (m *MemoryCacheAdapter) Close() error {
	return nil
}
