// middleware/cache_middleware.go
package middleware

import (
	//"ZZDNS/logger"
	"ZZDNS/shared"
	"fmt"
	"time"

	"github.com/miekg/dns"
)

func generateCacheKey(name string, qtype uint16) string {
	return fmt.Sprintf("%s_%d", name, qtype)
}

func CacheMiddleware(next dns.Handler) dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) == 0 {
			next.ServeDNS(w, r)
			return
		}
		// 生成缓存键
		key := generateCacheKey(r.Question[0].Name, r.Question[0].Qtype)
		// 检查缓存中是否存在响应
		if cachedResponse, exists := shared.DnsCache.Get(key); exists {
			cachedResponse.Id = r.Id  // 使用当前请求的 ID
			w.WriteMsg(cachedResponse) // 返回缓存的响应
			
			//logger.GetLogger().Info(fmt.Sprintf("Cache hit: domain=%s, qtype=%d, id=%d", r.Question[0].Name, r.Question[0].Qtype, r.Id))
			return
		}

		next.ServeDNS(w, r)
	})
}

func StartCacheCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			shared.DnsCache.Cleanup() // 使用共享的 DNS 缓存
		}
	}()
}