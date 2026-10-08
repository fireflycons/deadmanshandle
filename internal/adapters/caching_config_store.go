package adapters

import (
	"context"
	"sync"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// CachingConfigStore keeps the configuration from another ConfigStore for ttl,
// so that a warm Lambda does not fetch (and decrypt) it on every request.
// Errors are not cached.
type CachingConfigStore struct {
	inner ports.ConfigStore
	ttl   time.Duration
	now   func() time.Time

	mu        sync.Mutex
	name      string
	data      []byte
	fetchedAt time.Time
}

// NewCachingConfigStore wraps inner with a cache that holds a value for ttl
func NewCachingConfigStore(inner ports.ConfigStore, ttl time.Duration) *CachingConfigStore {
	return &CachingConfigStore{
		inner: inner,
		ttl:   ttl,
		now:   time.Now,
	}
}

// GetConfig returns the cached configuration if it is for the same parameter
// and younger than ttl, otherwise fetches it from the inner store
func (c *CachingConfigStore) GetConfig(ctx context.Context, parameterName string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	if c.data != nil && c.name == parameterName && now.Sub(c.fetchedAt) < c.ttl {
		return c.data, nil
	}

	data, err := c.inner.GetConfig(ctx, parameterName)
	if err != nil {
		return nil, err
	}

	c.name, c.data, c.fetchedAt = parameterName, data, now
	return data, nil
}
