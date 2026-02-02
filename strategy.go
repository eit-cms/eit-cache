package eitcache

import (
	"context"
	"log"
	"sync"
	"time"
)

// SmartCacheStrategy refreshes cache before TTL expires.
type SmartCacheStrategy struct {
	ttl            time.Duration
	updateInterval time.Duration
	lastUpdate     time.Time
	mu             sync.RWMutex
}

// NewSmartCacheStrategy creates a strategy.
func NewSmartCacheStrategy(ttl time.Duration) *SmartCacheStrategy {
	interval := ttl / 2
	if interval <= 0 {
		interval = time.Minute
	}
	return &SmartCacheStrategy{
		ttl:            ttl,
		updateInterval: interval,
		lastUpdate:     time.Now(),
	}
}

// ShouldRefresh returns true if cache should refresh.
func (s *SmartCacheStrategy) ShouldRefresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return time.Since(s.lastUpdate) > s.updateInterval
}

// MarkUpdated marks cache as refreshed.
func (s *SmartCacheStrategy) MarkUpdated() {
	s.mu.Lock()
	s.lastUpdate = time.Now()
	s.mu.Unlock()
}

// PrefetchCacheStrategy preloads cache pages.
type PrefetchCacheStrategy struct {
	enabled   bool
	batchSize int
	mu        sync.RWMutex
}

// NewPrefetchCacheStrategy creates a prefetch strategy.
func NewPrefetchCacheStrategy(enabled bool, batchSize int) *PrefetchCacheStrategy {
	if batchSize <= 0 {
		batchSize = 1
	}
	return &PrefetchCacheStrategy{
		enabled:   enabled,
		batchSize: batchSize,
	}
}

// Prefetch loads paginated cache entries.
func Prefetch[T any](
	ctx context.Context,
	strategy *PrefetchCacheStrategy,
	manager *Manager,
	resource string,
	items []T,
	pageSize int,
	ttl time.Duration,
) error {
	if manager == nil || strategy == nil || !strategy.enabled {
		return nil
	}

	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	totalPages := (len(items) + pageSize - 1) / pageSize
	if totalPages > strategy.batchSize {
		totalPages = strategy.batchSize
	}

	for page := 1; page <= totalPages; page++ {
		offset := (page - 1) * pageSize
		end := offset + pageSize
		if end > len(items) {
			end = len(items)
		}
		pageData := items[offset:end]
		params := &PaginationParams{Page: page, PageSize: pageSize, UseCache: true}
		cacheKey := GenerateCacheKey(resource, nil, params)
		if err := manager.Set(ctx, cacheKey, paginationCacheItem[T]{
			Data:     pageData,
			Total:    int64(len(items)),
			DataHash: GenerateDataHash(pageData),
		}, ttl); err != nil {
			log.Printf("[CACHE] prefetch failed (%s): %v", cacheKey, err)
		}
	}

	return nil
}

// CacheWarmer periodically refreshes cached data.
type CacheWarmer struct {
	manager *Manager
	jobs    map[string]func(context.Context) (interface{}, error)
	interval time.Duration
	stopChan chan struct{}
	mu       sync.RWMutex
}

// NewCacheWarmer creates a cache warmer.
func NewCacheWarmer(manager *Manager, interval time.Duration) *CacheWarmer {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &CacheWarmer{
		manager:  manager,
		jobs:     make(map[string]func(context.Context) (interface{}, error)),
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// AddJob registers a warmup job.
func (w *CacheWarmer) AddJob(key string, job func(context.Context) (interface{}, error)) {
	w.mu.Lock()
	w.jobs[key] = job
	w.mu.Unlock()
}

// RemoveJob removes a warmup job.
func (w *CacheWarmer) RemoveJob(key string) {
	w.mu.Lock()
	delete(w.jobs, key)
	w.mu.Unlock()
}

// Start begins warming.
func (w *CacheWarmer) Start() {
	if w == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.warmup()
			case <-w.stopChan:
				return
			}
		}
	}()
}

func (w *CacheWarmer) warmup() {
	if w.manager == nil {
		return
	}
	w.mu.RLock()
	jobs := make(map[string]func(context.Context) (interface{}, error), len(w.jobs))
	for k, v := range w.jobs {
		jobs[k] = v
	}
	w.mu.RUnlock()

	ctx := context.Background()
	for key, job := range jobs {
		data, err := job(ctx)
		if err != nil {
			log.Printf("[CACHE] warmup job failed (%s): %v", key, err)
			continue
		}
		if err := w.manager.Set(ctx, key, data, 0); err != nil {
			log.Printf("[CACHE] warmup set failed (%s): %v", key, err)
		}
	}
}

// Stop stops warming.
func (w *CacheWarmer) Stop() {
	if w == nil {
		return
	}
	close(w.stopChan)
}
