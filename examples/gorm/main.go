package main

import (
	"context"
	"fmt"
	"time"

	"github.com/eit-cms/eit-cache"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	ID   uint
	Name string
}

func main() {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	if err := db.AutoMigrate(&User{}); err != nil {
		panic(err)
	}

	_ = db.Create(&User{Name: "Alice"}).Error
	_ = db.Create(&User{Name: "Bob"}).Error

	manager, err := eitcache.NewManager(&eitcache.CacheConfig{
		Type:       eitcache.CacheTypeMemory,
		DefaultTTL: 5 * time.Minute,
	})
	if err != nil {
		panic(err)
	}
	defer manager.Close()

	ctx := context.Background()

	users, err := eitcache.Query(ctx, manager, "users:all", func() ([]User, error) {
		var result []User
		if err := db.Find(&result).Error; err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("users: %+v\n", users)
}
