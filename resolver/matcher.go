package resolver

import (
	"ZZDNS/config"
	//"ZZDNS/logger"
	"strings"
)

// GetUpstreamServer 返回给定域名的上游服务器
func GetUpstreamServer(domain string) (string, bool) {
	domain = strings.TrimSuffix(domain, ".")

	// 分割域名部分
	parts := strings.Split(domain, ".")

	// 从最精确的子域开始，逐步向上匹配
	for i := range parts {
		subDomain := strings.Join(parts[i:], ".")
		if server, found := config.DomainTrie.MatchDomain(subDomain); found {
			// 记录匹配结果
			//logger.GetLogger().Info("Matched Domain: " + subDomain + ", Upstream Server: " + server)
			return server, true
		}
	}

	// 如果没有找到匹配的服务器，返回默认服务器
	defaultServer := config.CFG.Server.DefaultServer
	//logger.GetLogger().Info("No exact match found. Using Default Server: " + defaultServer)
	return defaultServer, false
}
