// server/server.go
package server

import (
	"ZZDNS/config"
	"ZZDNS/logger"
	"ZZDNS/middleware"
	"ZZDNS/protocol"
	"ZZDNS/resolver"
	"ZZDNS/shared"
	"ZZDNS/utils"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/miekg/dns"
)

var cfgPath string

func StartServer(path string) {
	cfgPath = path
	go StartApiServer()
	StartDNSServer()
}

func StartApiServer() {
	http.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		if err := config.LoadConfig(cfgPath); err != nil {
			logger.GetLogger().Error(fmt.Sprintf("reload failed: %v", err))
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("fail"))
			return
		}
		logger.GetLogger().Info("config reloaded via API")
		_, _ = w.Write([]byte("ok"))
	})
	log.Printf("Starting API server on %s\n", config.CFG.Api.Port)
	if err := http.ListenAndServe(config.CFG.Api.Port, nil); err != nil {
		log.Fatalf("api server error: %v", err)
	}
}

type upstreamResult struct {
	resp   *dns.Msg
	server string
}

func queryAllUpstreams(r *dns.Msg) (*dns.Msg, string, error) {
	ch := make(chan upstreamResult, len(config.CFG.Forward))
	for _, f := range config.CFG.Forward {
		f := f
		go func() {
			req := r.Copy()
			proto, addr := resolver.ParseUpstreamServer(f.Server)
			resp, err := sendRequest(proto, req, addr)
			if err == nil {
				ch <- upstreamResult{resp: resp, server: f.Server}
			} else {
				ch <- upstreamResult{resp: nil, server: f.Server}
			}
		}()
	}

	var domestic *upstreamResult
	var foreign *upstreamResult

	for i := 0; i < len(config.CFG.Forward); i++ {
		res := <-ch
		if res.resp == nil {
			continue
		}
		isDomestic := false
		for _, ans := range res.resp.Answer {
			switch ans.Header().Rrtype {
			case dns.TypeA:
				if utils.CheckIPInSet(ans.(*dns.A).A.String(), config.CFG.Ipset.Name4) {
					isDomestic = true
				}
			case dns.TypeAAAA:
				if utils.CheckIPInSet(ans.(*dns.AAAA).AAAA.String(), config.CFG.Ipset.Name6) {
					isDomestic = true
				}
			}
		}
		if isDomestic && domestic == nil {
			tmp := res
			domestic = &tmp
		} else if !isDomestic && foreign == nil {
			tmp := res
			foreign = &tmp
		}
	}

	if domestic != nil {
		return domestic.resp, domestic.server, nil
	}
	if foreign != nil {
		return foreign.resp, foreign.server, nil
	}
	return nil, "", fmt.Errorf("no upstream response")
}
func StartDNSServer() {
	handler := resolver.Chain(
		middleware.CacheMiddleware, // 添加缓存中间件
		middleware.BlocklistMiddleware,
	)(dns.HandlerFunc(forwardToUpstream))

	server := &dns.Server{Addr: config.CFG.Server.Port, Net: "udp", Handler: handler, ReusePort: true}

	// 启动清理过期缓存的定时器
	utils.StartCacheCleanup(24 * time.Hour)

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

	// 初始化响应消息
	response := new(dns.Msg)
	response.SetReply(r)

	originalName := r.Question[0].Name
	qtype := r.Question[0].Qtype
	cnameChain := map[string]bool{} // 用于记录已经解析过的 CNAME 记录
	var minTTL uint32 = 0           // 用于记录最小的 TTL
	var logRecords []string
	var cnameAddressChain []string
	var addressChain []string

	maxCnameChainLength := 10 // 限制 CNAME 链长度，防止无限循环

	for i := 0; i < maxCnameChainLength; i++ {
		upstreamServer, matched := resolver.GetUpstreamServer(r.Question[0].Name)
		var upstreamResponse *dns.Msg
		var err error
		var address string
		if matched {
			protocolType, addr := resolver.ParseUpstreamServer(upstreamServer)
			address = addr

			// 检查是否支持 IPv6 查询
			if qtype == dns.TypeAAAA && !resolver.IsUpstreamIPv6Supported(upstreamServer) {
				dns.HandleFailed(w, r)
				return
			}

			addressChain = append(addressChain, address)
			upstreamResponse, err = sendRequest(protocolType, r, addr)
			if err != nil || upstreamResponse == nil || upstreamResponse.Answer == nil {
				dns.HandleFailed(w, r)
				return
			}
		} else {
			upstreamResponse, upstreamServer, err = queryAllUpstreams(r)
			if err != nil {
				dns.HandleFailed(w, r)
				return
			}
			_, address = resolver.ParseUpstreamServer(upstreamServer)
			addressChain = append(addressChain, address)
		}

		foundNonCNAME := false
		foundNewCNAME := false

		for _, ans := range upstreamResponse.Answer {
			ttl := ans.Header().Ttl
			if minTTL == 0 || ttl < minTTL {
				minTTL = ttl
			}

			switch ans.Header().Rrtype {
			case dns.TypeA:
				ip := ans.(*dns.A).A.String()
				logRecords = append(logRecords, ip)
				foundNonCNAME = true

			case dns.TypeAAAA:
				ip := ans.(*dns.AAAA).AAAA.String()
				logRecords = append(logRecords, ip)
				foundNonCNAME = true

			case dns.TypeCNAME:
				cname := ans.(*dns.CNAME).Target
				// 如果这个 cname 还没解析过，就准备下一轮继续查询它
				if !cnameChain[cname] {
					cnameChain[cname] = true
					cnameAddressChain = append(cnameAddressChain, cname)
					logRecords = append(logRecords, cname)
					r.Question[0].Name = cname
					foundNewCNAME = true
				}
			}

			// 不管是 CNAME 还是 A/AAAA，都要加到 response.Answer
			response.Answer = append(response.Answer, ans)
		}

		// 如果已经找到 A/AAAA，则不再继续递归
		if foundNonCNAME {
			break
		}

		// 如果这一轮没有发现新的 CNAME，说明都解析过或者没有可递归的 CNAME，可退出
		if !foundNewCNAME {
			break
		}
	}

	// 如果达到最大 CNAME 链长度，认为可能出现循环，返回错误
	if len(cnameChain) == maxCnameChainLength {
		log.Println("CNAME chain too long, possible loop detected")
		dns.HandleFailed(w, r)
		return
	}

	// 在发送响应之前，缓存非错误的响应 (RcodeSuccess)
	if response.Rcode == dns.RcodeSuccess {
		if minTTL == 0 {
			// 如果根本没找到任何记录，minTTL 还是 0，可以根据需要设一个缺省值或不缓存
			minTTL = 0 // 也可以自定义
		}
		ttl := time.Duration(minTTL) * time.Second
		cacheKey := utils.GenerateCacheKey(originalName, qtype)
		shared.DnsCache.Set(cacheKey, response, ttl)
	}

	// 记录 IP / CNAME / CNAME 链 / 上游服务器链
	if len(logRecords) > 0 {
		joinedLogRecords := strings.Join(logRecords, ", ")
		joinedCnameAddressChain := strings.Join(cnameAddressChain, " -> ")
		joinedAddressChain := strings.Join(addressChain, " -> ")
		logger.GetLogger().Info(fmt.Sprintf("Source IP: %s, Query: %s, Address Chain: %s, Results: %s, CNAME Chain: %s",
			srcIP, originalName, joinedAddressChain, joinedLogRecords, joinedCnameAddressChain))
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
