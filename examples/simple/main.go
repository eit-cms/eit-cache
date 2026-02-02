package main

import (
	"context"
	"fmt"
	"time"

	"github.com/eit-cms/eit-cache"
)

type Profile struct {
	ID   int
	Name string
}

func main() {
	manager, err := eitcache.NewManager(&eitcache.CacheConfig{
		Type:       eitcache.CacheTypeMemory,
		DefaultTTL: 2 * time.Minute,
	})
	if err != nil {
		panic(err)
	}
	defer manager.Close()

	ctx := context.Background()

	profile, err := eitcache.Query(ctx, manager, "profile:42", func() (Profile, error) {
		return Profile{ID: 42, Name: "Ada"}, nil
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("profile: %+v\n", profile)
}
