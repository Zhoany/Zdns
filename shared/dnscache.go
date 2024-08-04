// shared/dnscache.go
package shared

import (
	"ZZDNS/cache"
)

var DnsCache = cache.NewDnsCache() // 实例化共享的 DNS 缓存