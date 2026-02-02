# eit-cache

通用 Go 缓存库，支持 Redis 与内存两种后端，提供简洁的缓存查询 API，可与 GORM 或 eit-db 组合使用。

## 安装

```bash
go get github.com/DeathCodeBind/eit-cache
```

## 快速开始

### 内存缓存

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/DeathCodeBind/eit-cache"
)

type Profile struct {
	ID   int
	Name string
}

func main() {
	manager, _ := eitcache.NewManager(&eitcache.CacheConfig{
		Type:       eitcache.CacheTypeMemory,
		DefaultTTL: 2 * time.Minute,
	})
	defer manager.Close()

	ctx := context.Background()
	profile, _ := manager.Query(ctx, "profile:42", func() (Profile, error) {
		return Profile{ID: 42, Name: "Ada"}, nil
	})

	fmt.Printf("%+v\n", profile)
}
```

### Redis 缓存

```go
manager, err := eitcache.NewManager(&eitcache.CacheConfig{
	Type:       eitcache.CacheTypeRedis,
	Addr:       "localhost:6379",
	Password:   "",
	DB:         0,
	DefaultTTL: 5 * time.Minute,
})
```

## API 文档

### Manager

- `NewManager(config *CacheConfig) (*Manager, error)`
- `NewManagerWithAdapter(adapter Adapter, defaultTTL time.Duration) *Manager`
- `Query[T any](ctx context.Context, key string, queryFunc func() (T, error), opts ...QueryOption) (T, error)`
- `Get(ctx context.Context, key string, dest interface{}) (bool, error)`
- `Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error`
- `Delete(ctx context.Context, keys ...string) error`
- `DeletePattern(ctx context.Context, pattern string) (int64, error)`
- `Exists(ctx context.Context, key string) (bool, error)`
- `Stats(ctx context.Context) (map[string]interface{}, error)`
- `Ping(ctx context.Context) error`
- `Close() error`
- `Monitor() *Monitor`

### Query 选项

- `WithTTL(ttl time.Duration)`
- `WithNoCache()`
- `WithTicket(ticket *CacheTicket)`

### Adapter

- `RedisCacheAdapter`（Redis 后端）
- `MemoryCacheAdapter`（内存后端）
- `AdvancedRedisCacheAdapter`（带监控的 Redis 适配器）

### Ticket

- `GenerateTicket(userID string, ttl time.Duration) *CacheTicket`
- `(*CacheTicket).Validate() error`

### Pagination

- `PaginationParams` / `PaginationResponse[T any]`
- `NormalizePaginationParams(params *PaginationParams) *PaginationParams`
- `GenerateCacheKey(resource string, filters map[string]interface{}, params *PaginationParams) string`
- `GenerateDataHash(data interface{}) string`
- `QueryWithPagination[T any](ctx context.Context, resource string, filters map[string]interface{}, params *PaginationParams, queryFunc func() ([]T, int64, error)) (*PaginationResponse[T], error)`
- `QueryWithCache[T any](ctx context.Context, manager *Manager, resource string, filters map[string]interface{}, params *PaginationParams, queryFunc func() ([]T, int64, error)) (*PaginationResponse[T], error)`
- `InvalidateCacheOnUpdate(ctx context.Context, manager *Manager, resource string) (int64, error)`

### Monitor

- `Monitor` / `CacheMetrics`
- `(*Monitor).RecordHit(duration time.Duration)`
- `(*Monitor).RecordMiss(duration time.Duration)`
- `(*Monitor).HitRatio() float64`
- `(*Monitor).GetMetrics() CacheMetrics`
- `(*Monitor).Reset()`

### Strategy & Warmup

- `SmartCacheStrategy`
- `PrefetchCacheStrategy`
- `CacheWarmer`
- `CacheCompression`

## 示例

- 简单示例：examples/simple
- GORM 示例：examples/gorm
- eit-db 示例：examples/eitdb（需要构建标签 `eitdb`）

运行 eit-db 示例：

```bash
go run -tags eitdb ./examples/eitdb
```

## 与 eit-db 集成

`eit-cache` 不依赖 ORM，可直接与 eit-db 的 `Repository`/`QueryBuilder` 组合使用。推荐用法是在缓存 `Query` 的 `queryFunc` 中调用 eit-db 的查询逻辑。
