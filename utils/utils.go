package utils

import (
	"ZZDNS/shared"
	"fmt"
	"time"
)

func GenerateCacheKey(name string, qtype uint16) string {
	return fmt.Sprintf("%s_%d", name, qtype)
}
func StartCacheCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			shared.DnsCache.Cleanup() 
		}
	}()
}