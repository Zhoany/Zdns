package resolver

import "strings"

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
