package eitcache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCache(t *testing.T) {
	manager, err := NewManager(&CacheConfig{
		Type:       CacheTypeMemory,
		DefaultTTL: 1 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	ctx := context.Background()

	type User struct {
		ID   int
		Name string
	}

	user := User{ID: 1, Name: "Test"}
	if err := manager.Set(ctx, "user:1", user, 1*time.Minute); err != nil {
		t.Fatal(err)
	}

	var retrieved User
	hit, err := manager.Get(ctx, "user:1", &retrieved)
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Fatal("expected cache hit")
	}
	if retrieved.ID != user.ID || retrieved.Name != user.Name {
		t.Fatal("data mismatch")
	}

	if err := manager.Delete(ctx, "user:1"); err != nil {
		t.Fatal(err)
	}

	hit, _ = manager.Get(ctx, "user:1", &retrieved)
	if hit {
		t.Fatal("expected cache miss after delete")
	}
}

func TestQuery(t *testing.T) {
	manager, err := NewManager(&CacheConfig{
		Type:       CacheTypeMemory,
		DefaultTTL: 1 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	ctx := context.Background()

	type User struct {
		ID   int
		Name string
	}

	result, err := Query(ctx, manager, "user:2", func() (User, error) {
		return User{ID: 2, Name: "Ada"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != 2 || result.Name != "Ada" {
		t.Fatal("query result mismatch")
	}
}

func TestPagination(t *testing.T) {
	manager, err := NewManager(&CacheConfig{
		Type:       CacheTypeMemory,
		DefaultTTL: 1 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	ctx := context.Background()

	type Item struct {
		ID   int
		Name string
	}

	items := []Item{
		{ID: 1, Name: "Item1"},
		{ID: 2, Name: "Item2"},
		{ID: 3, Name: "Item3"},
	}

	resp, err := QueryWithPagination(ctx, manager, "items", nil, &PaginationParams{
		Page:     1,
		PageSize: 2,
		UseCache: true,
	}, func() ([]Item, int64, error) {
		return items[:2], int64(len(items)), nil
	})

	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 3 || len(resp.Data) != 2 {
		t.Fatalf("expected 3 total, 2 on page, got %d total, %d data", resp.Total, len(resp.Data))
	}
	if resp.TotalPages != 2 {
		t.Fatalf("expected 2 pages, got %d", resp.TotalPages)
	}
}

func TestTicket(t *testing.T) {
	ticket := GenerateTicket("user123", 1*time.Hour)
	if ticket == nil || ticket.Token == "" {
		t.Fatal("failed to generate ticket")
	}

	if err := ticket.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMonitor(t *testing.T) {
	monitor := NewMonitor()

	monitor.RecordHit(10 * time.Millisecond)
	monitor.RecordMiss(20 * time.Millisecond)
	monitor.RecordHit(15 * time.Millisecond)

	ratio := monitor.HitRatio()
	if ratio != 2.0/3.0 {
		t.Fatalf("expected 2/3 hit ratio, got %f", ratio)
	}
}
