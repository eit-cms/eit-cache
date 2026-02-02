package eitcache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	DefaultPageSize = 20
	DefaultPage     = 1
	MaxPageSize     = 200
)

// PaginationParams defines pagination options.
type PaginationParams struct {
	Page     int  `json:"page"`
	PageSize int  `json:"page_size"`
	UseCache bool `json:"use_cache"`
}

// NormalizePaginationParams returns normalized params.
func NormalizePaginationParams(params *PaginationParams) *PaginationParams {
	if params == nil {
		return &PaginationParams{
			Page:     DefaultPage,
			PageSize: DefaultPageSize,
			UseCache: true,
		}
	}
	if params.Page <= 0 {
		params.Page = DefaultPage
	}
	if params.PageSize <= 0 {
		params.PageSize = DefaultPageSize
	}
	if params.PageSize > MaxPageSize {
		params.PageSize = MaxPageSize
	}
	return params
}

// PaginationResponse represents a paginated response.
type PaginationResponse[T any] struct {
	Data       []T    `json:"data"`
	Total      int64  `json:"total"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	TotalPages int    `json:"total_pages"`
	FromCache  bool   `json:"from_cache"`
	CacheKey   string `json:"cache_key"`
	DataHash   string `json:"data_hash"`
}

type paginationCacheItem[T any] struct {
	Data     []T   `json:"data"`
	Total    int64 `json:"total"`
	DataHash string `json:"data_hash"`
}

// BuildPaginationResponse builds response with computed fields.
func BuildPaginationResponse[T any](data []T, total int64, params *PaginationParams, cacheKey string, fromCache bool) *PaginationResponse[T] {
	params = NormalizePaginationParams(params)
	pages := int((total + int64(params.PageSize) - 1) / int64(params.PageSize))
	return &PaginationResponse[T]{
		Data:       data,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: pages,
		FromCache:  fromCache,
		CacheKey:   cacheKey,
		DataHash:   GenerateDataHash(data),
	}
}

// GenerateCacheKey builds a stable cache key with filters and pagination.
func GenerateCacheKey(resource string, filters map[string]interface{}, params *PaginationParams) string {
	params = NormalizePaginationParams(params)
	parts := []string{resource}

	if len(filters) > 0 {
		keys := make([]string, 0, len(filters))
		for k := range filters {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			val, _ := json.Marshal(filters[k])
			parts = append(parts, k, string(val))
		}
	}

	parts = append(parts, "page", fmt.Sprintf("%d", params.Page), "size", fmt.Sprintf("%d", params.PageSize))
	return strings.Join(parts, ":")
}

// GenerateDataHash hashes data for comparison.
func GenerateDataHash(data interface{}) string {
	payload, _ := json.Marshal(data)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// QueryWithPagination executes a cached paginated query.
func QueryWithPagination[T any](
	ctx context.Context,
	manager *Manager,
	resource string,
	filters map[string]interface{},
	params *PaginationParams,
	queryFunc func() ([]T, int64, error),
) (*PaginationResponse[T], error) {
	if manager == nil {
		return nil, ErrManagerNil
	}
	params = NormalizePaginationParams(params)
	key := GenerateCacheKey(resource, filters, params)

	if params.UseCache {
		data, err := manager.adapter.Get(ctx, key)
		if err == nil && data != nil {
			var cached paginationCacheItem[T]
			if err := json.Unmarshal(data, &cached); err == nil {
				resp := BuildPaginationResponse(cached.Data, cached.Total, params, key, true)
				resp.DataHash = cached.DataHash
				return resp, nil
			}
		}
	}

	data, total, err := queryFunc()
	if err != nil {
		return nil, err
	}

	resp := BuildPaginationResponse(data, total, params, key, false)
	if params.UseCache {
		_ = manager.adapter.Set(ctx, key, paginationCacheItem[T]{
			Data:     data,
			Total:    total,
			DataHash: resp.DataHash,
		}, manager.defaultTTL)
	}

	return resp, nil
}

// QueryWithCache is a helper for cached pagination queries.
func QueryWithCache[T any](
	ctx context.Context,
	manager *Manager,
	resource string,
	filters map[string]interface{},
	params *PaginationParams,
	queryFunc func() ([]T, int64, error),
) (*PaginationResponse[T], error) {
	if manager == nil {
		return nil, ErrManagerNil
	}
	return QueryWithPagination(ctx, manager, resource, filters, params, queryFunc)
}

// InvalidateCacheOnUpdate clears cache entries for a resource.
func InvalidateCacheOnUpdate(ctx context.Context, manager *Manager, resource string) (int64, error) {
	if manager == nil {
		return 0, ErrManagerNil
	}
	pattern := resource + ":"
	return manager.DeletePattern(ctx, pattern)
}
