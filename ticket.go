package eitcache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidTicket = errors.New("invalid ticket")
	ErrTicketExpired = errors.New("ticket expired")
)

// CacheTicket provides user-scoped cache token.
type CacheTicket struct {
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GenerateTicket creates a cache ticket.
func GenerateTicket(userID string, ttl time.Duration) *CacheTicket {
	now := time.Now()
	src := fmt.Sprintf("%s:%d:%d", userID, now.Unix(), now.Nanosecond())
	hash := sha256.Sum256([]byte(src))
	return &CacheTicket{
		UserID:    userID,
		Token:     hex.EncodeToString(hash[:])[:16],
		IssuedAt:  now,
		ExpiresAt: now.Add(ttl),
	}
}

// Validate checks ticket validity.
func (t *CacheTicket) Validate() error {
	if t == nil || t.Token == "" {
		return ErrInvalidTicket
	}
	if time.Now().After(t.ExpiresAt) {
		return ErrTicketExpired
	}
	return nil
}
