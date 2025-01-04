// middleware/cache_middleware.go
package middleware

import (
	"ZZDNS/logger"
	"ZZDNS/shared"
	"ZZDNS/utils"
	"fmt"

	"log"
	"net"

	"github.com/miekg/dns"
)


func CacheMiddleware(next dns.Handler) dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) == 0 {
			next.ServeDNS(w, r)
			return
		}
		srcIP, _, err := net.SplitHostPort(w.RemoteAddr().String())
	if err != nil {
		log.Printf("Failed to get source IP: %v", err)
		dns.HandleFailed(w, r)
		return
	}
		// 生成缓存键
		key := utils.GenerateCacheKey(r.Question[0].Name, r.Question[0].Qtype)
		// 检查缓存中是否存在响应
		if cachedResponse, exists := shared.DnsCache.Get(key); exists {
			cachedResponse.Id = r.Id  // 使用当前请求的 ID
			w.WriteMsg(cachedResponse) // 返回缓存的响应
			
			logger.GetLogger().Info(fmt.Sprintf("Source IP: %s,Cache hit: domain=%s",srcIP, r.Question[0].Name))
			return
		}

		next.ServeDNS(w, r)
	})
}

