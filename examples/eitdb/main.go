//go:build eitdb
// +build eitdb

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/eit-cms/eit-cache"
	db "github.com/eit-cms/eit-db"
)

type User struct {
	ID   uint
	Name string
}

func main() {
	repo, err := db.NewRepository(&db.Config{
		Adapter:  "sqlite",
		Database: "file::memory:?cache=shared",
	})
	if err != nil {
		panic(err)
	}

	gormDB := repo.GetGormDB()
	if err := gormDB.AutoMigrate(&User{}); err != nil {
		panic(err)
	}
	_ = gormDB.Create(&User{Name: "Ada"}).Error
	_ = gormDB.Create(&User{Name: "Linus"}).Error

	manager, err := eitcache.NewManager(&eitcache.CacheConfig{
		Type:       eitcache.CacheTypeMemory,
		DefaultTTL: 5 * time.Minute,
	})
	if err != nil {
		panic(err)
	}
	defer manager.Close()

	ctx := context.Background()
	users, err := eitcache.Query(ctx, manager, "users:eitdb", func() ([]User, error) {
		var result []User
		if err := repo.GetGormDB().Find(&result).Error; err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("users: %+v\n", users)
}

























































