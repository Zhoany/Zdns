package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type CacheItem struct {
	Response   *dns.Msg
	Expiration time.Time
}

type DnsCache struct {
	items map[string]CacheItem
	mu    sync.RWMutex
}

func NewDnsCache() *DnsCache {
	return &DnsCache{
		items: make(map[string]CacheItem),
	}
}

// Get retrieves an item from the cache. If the item does not exist or is expired, it returns false.
func (c *DnsCache) Get(key string) (*dns.Msg, bool) {
	c.mu.RLock()
	item, exists := c.items[key]
	c.mu.RUnlock()

	if !exists || time.Now().After(item.Expiration) {
		// If the item exists but is expired, delete it
		if exists {
			c.mu.Lock()
			delete(c.items, key)
			c.mu.Unlock()
		}
		return nil, false
	}

	// Return a copy of the cached response to prevent external modification
	return item.Response.Copy(), true
}

// Set stores an item in the cache with the specified TTL.
func (c *DnsCache) Set(key string, response *dns.Msg, ttl time.Duration) {
	expiration := time.Now().Add(ttl)
	cacheItem := CacheItem{
		Response:   response.Copy(), // Ensure data safety by storing a copy
		Expiration: expiration,
	}

	c.mu.Lock()
	c.items[key] = cacheItem
	c.mu.Unlock()
}

// Cleanup removes expired items from the cache.
func (c *DnsCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.Expiration) {
			delete(c.items, key)
		}
	}
}
func (c *DnsCache) PrintAllItems() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for key, item := range c.items {
		fmt.Printf("Key: %s, Name: %s, Type: %d, TTL: %d, Expiration: %s\n",
			key, item.Response.Question[0].Name, item.Response.Question[0].Qtype,
			item.Response.Answer[0].Header().Ttl, item.Expiration)
	}
}