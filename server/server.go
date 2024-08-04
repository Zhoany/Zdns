// server/server.go
package server

import (
	"ZZDNS/config"
	"ZZDNS/logger"
	"ZZDNS/middleware"
	"ZZDNS/protocol"
	"ZZDNS/resolver"
	"ZZDNS/shared"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func generateCacheKey(name string, qtype uint16) string {
	return fmt.Sprintf("%s_%d", name, qtype)
}

func StartServer() {
	handler := resolver.Chain(
		middleware.CacheMiddleware, // 添加缓存中间件
		middleware.BlocklistMiddleware,
	)(dns.HandlerFunc(forwardToUpstream))

	server := &dns.Server{Addr: config.CFG.Server.Port, Net: "udp", Handler: handler}

	// 启动清理过期缓存的定时器
	middleware.StartCacheCleanup(5 * time.Minute)

	// 启动服务器
	log.Printf("Starting DNS server on %s\n", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start DNS server: %v\n", err)
	}
}

// 处理 DNS 查询
func forwardToUpstream(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		dns.HandleFailed(w, r)
		return
	}

	// 获取源IP
	srcIP, _, err := net.SplitHostPort(w.RemoteAddr().String())
	if err != nil {
		log.Printf("Failed to get source IP: %v", err)
		dns.HandleFailed(w, r)
		return
	}

	// 检查是否支持 IPv6
	if !config.CFG.Server.IPv6 {
		for _, question := range r.Question {
			if question.Qtype == dns.TypeAAAA {
				dns.HandleFailed(w, r)
				return
			}
		}
	}

	// 初始化响应消息
	response := new(dns.Msg)
	response.SetReply(r)

	originalName := r.Question[0].Name
	qtype := r.Question[0].Qtype
	cnameChain := map[string]bool{} // 用于记录已经解析过的CNAME记录
	var minTTL uint32 = 0           // 用于记录最小的TTL
	var upstreamServer string       // 记录上游服务器
	var logRecords []string         // 用于记录IP地址或CNAME
	var cnameAddressChain []string  // 用于记录整个CNAME地址链
	var addressChain []string       // 用于记录上游服务器地址链

	// 限制CNAME链长度，防止无限循环
	maxCnameChainLength := 10
	cnameFound := true

	for i := 0; i < maxCnameChainLength && cnameFound; i++ {
		cnameFound = false
		// 获取上游服务器信息
		upstreamServer = resolver.GetUpstreamServer(r.Question[0].Name)
		protocolType, address := resolver.ParseUpstreamServer(upstreamServer)
		addressChain = append(addressChain, address)
		upstreamResponse, err := sendRequest(protocolType, r, address)
		if err != nil {
			dns.HandleFailed(w, r)
			return
		}

		for _, ans := range upstreamResponse.Answer {
			ttl := ans.Header().Ttl
			if minTTL == 0 || ttl < minTTL {
				minTTL = ttl
			}

			switch ans.Header().Rrtype {
			case dns.TypeA:
				ip := ans.(*dns.A).A.String()
				logRecords = append(logRecords, ip)
			case dns.TypeAAAA:
				ip := ans.(*dns.AAAA).AAAA.String()
				logRecords = append(logRecords, ip)
			case dns.TypeCNAME:
				cname := ans.(*dns.CNAME).Target
				if _, exists := cnameChain[cname]; exists {
					continue
				}
				cnameChain[cname] = true
				cnameAddressChain = append(cnameAddressChain, cname)
				logRecords = append(logRecords, cname)
				r.Question[0].Name = cname
				cnameFound = true
			}

			response.Answer = append(response.Answer, ans)
		}

		// 如果找到的答案不只是 CNAME 记录，则不继续递归
		if len(response.Answer) > 1 {
			break
		}
	}

	// 如果达到最大CNAME链长度，认为有可能是循环，处理错误
	if len(cnameChain) == maxCnameChainLength {
		log.Println("CNAME chain too long, possible loop detected")
		dns.HandleFailed(w, r)
		return
	}

	// 在发送响应之前，缓存非错误的响应
	if response.Rcode == dns.RcodeSuccess {
		ttl := time.Duration(minTTL) * time.Second
		cacheKey := generateCacheKey(originalName, qtype)
		shared.DnsCache.Set(cacheKey, response, ttl) // 使用共享的 DNS 缓存
	}

	// 记录 IP 或 CNAME 和 CNAME 链
	if len(logRecords) > 0 {
		joinedLogRecords := strings.Join(logRecords, ", ")
		joinedCnameAddressChain := strings.Join(cnameAddressChain, " -> ")
		joinedAddressChain := strings.Join(addressChain, " -> ")
		logger.GetLogger().Info(fmt.Sprintf("Source IP: %s, Query: %s,Address Chain: %s, Results: %s, CNAME Chain: %s",
			srcIP, originalName,joinedAddressChain, joinedLogRecords, joinedCnameAddressChain))
	}

	response.RecursionAvailable = true
	w.WriteMsg(response)
}
func sendRequest(protocolType string, r *dns.Msg, address string) (*dns.Msg, error) {
	switch protocolType {
	case "udp":
		return protocol.UdpRequest(r, address)
	case "tls":
		return protocol.DoTRequest(r, address)
	case "https":
		return protocol.DoHRequest(r, address)
	default:
		msg := new(dns.Msg)
		msg.SetRcodeFormatError(r)
		return msg, nil
	}
}