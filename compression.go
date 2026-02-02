package eitcache

// CacheCompression determines whether to compress cache payloads.
type CacheCompression struct {
	Threshold int
}

// NewCacheCompression creates a compression policy.
func NewCacheCompression(threshold int) *CacheCompression {
	return &CacheCompression{Threshold: threshold}
}

// ShouldCompress reports if data length exceeds threshold.
func (c *CacheCompression) ShouldCompress(data []byte) bool {
	if c == nil {
		return false
	}
	return len(data) > c.Threshold
}
