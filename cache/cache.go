package cache

import (
	"github.com/dgraph-io/ristretto"
	"time"
)

type Cache struct {
	cache *ristretto.Cache
}

func NewCache(size int64) *Cache {
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: size,
		MaxCost:     size,
		BufferItems: 64,
	})
	c := &Cache{cache: cache}
	go c.cleanup()
	return c
}

func (c *Cache) Get(key string) (interface{}, bool) {
	return c.cache.Get(key)
}

func (c *Cache) Set(key string, value interface{}) bool {
	return c.cache.Set(key, value, 1)
}

// cleanup 定期清理过期缓存条目
func (c *Cache) cleanup() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		c.cache.Clear()
	}
}
