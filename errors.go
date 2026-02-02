package eitcache

import "errors"

var (
	ErrManagerNil  = errors.New("cache manager is nil")
	ErrInvalidType = errors.New("invalid cache type")
)
