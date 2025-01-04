package resolver

import (
	"ZZDNS/config"
	"fmt"
	"strings"
)

func ParseUpstreamServer(upstreamServer string) (protocol string, address string) {
	if strings.HasPrefix(upstreamServer, "https://") {
		return "https", upstreamServer
	}
	parts := strings.SplitN(upstreamServer, "://", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}


// IsUpstreamIPv6Supported 检查上游服务器是否支持 IPv6 查询
func IsUpstreamIPv6Supported(server string) bool {
	if server == config.CFG.Server.DefaultServer {
		fmt.Println("config.CFG.Server.V6")
        return config.CFG.Server.V6 
    }
	for _, forward := range config.CFG.Forward {
        if forward.Server == server {
			fmt.Println("config.CFG.Server.V6")
            return forward.V6 // 检查是否允许 IPv6
        }
    }
    return false
}
