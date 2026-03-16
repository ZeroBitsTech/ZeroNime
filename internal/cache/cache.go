package cache

import (
	"time"

	"github.com/jellydator/ttlcache/v3"
)

type Cache struct {
	store *ttlcache.Cache[string, any]
}

func New(ttl time.Duration) *Cache {
	store := ttlcache.New[string, any](ttlcache.WithTTL[string, any](ttl))
	go store.Start()
	return &Cache{store: store}
}

func (c *Cache) Get(key string) (any, bool) {
	if item := c.store.Get(key); item != nil {
		return item.Value(), true
	}
	return nil, false
}

func (c *Cache) Set(key string, value any, ttl time.Duration) {
	c.store.Set(key, value, ttl)
}

func (c *Cache) Delete(key string) {
	c.store.Delete(key)
}
