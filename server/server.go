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
	"log"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func DNSServer() {
	
	StartDNSServer()
}



func StartDNSServer() {
	handler := resolver.Chain(
		middleware.CacheMiddleware,

	)(dns.HandlerFunc(forwardToUpstream))

	// UDP Server
	udpServer := &dns.Server{
		Addr:      config.CFG.Server.Port,
		Net:       "udp",
		Handler:   handler,
		ReusePort: true,
	}
	// TCP Server
	tcpServer := &dns.Server{
		Addr:      config.CFG.Server.Port,
		Net:       "tcp",
		Handler:   handler,
		ReusePort: true,
	}

	// 启动定时缓存清理
	utils.StartCacheCleanup(24 * time.Hour)

	log.Printf("Starting DNS server on %s (udp)\n", config.CFG.Server.Port)
	go func() {
		if err := udpServer.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start UDP DNS server: %v\n", err)
		}
	}()

	log.Printf("Starting DNS server on %s (tcp)\n", config.CFG.Server.Port)
	if err := tcpServer.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start TCP DNS server: %v\n", err)
	}
}

func forwardToUpstream(w dns.ResponseWriter, r *dns.Msg) {
	start := time.Now()
	// 只支持单问
	if len(r.Question) != 1 {
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeFormatError)
		w.WriteMsg(msg)
		return
	}
	// 获取源IP
	srcIP, _, err := net.SplitHostPort(w.RemoteAddr().String())
	if err != nil {
		log.Printf("Failed to get source IP: %v\n", err)
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(msg)
		return
	}

	// 准备
	origName := r.Question[0].Name
	qtype := r.Question[0].Qtype
	// 检查是否在 hosts 文件中
	hostname := strings.TrimSuffix(origName, ".")

	ip, ok := config.Hosts[hostname]

	if ok {
		resp := new(dns.Msg)
		resp.SetReply(r)
		header := dns.RR_Header{Name: origName, Rrtype: qtype, Class: dns.ClassINET, Ttl: 300}
		parsed := net.ParseIP(ip)
		var rr dns.RR
		if qtype == dns.TypeA && parsed.To4() != nil {
			rr = &dns.A{Hdr: header, A: parsed.To4()}
		} else if qtype == dns.TypeAAAA && parsed.To16() != nil && parsed.To4() == nil {
			rr = &dns.AAAA{Hdr: header, AAAA: parsed}
		}
		if rr != nil {
			resp.Answer = []dns.RR{rr}
		}
		resp.RecursionAvailable = true
		logResponse(srcIP, origName, []string{"HOST"}, resp, start)
		w.WriteMsg(resp)
		return
	}

	// 如果 AAAA 不支持，返回 NotImplemented
	firstUp := resolver.GetUpstreamServer(origName)
	if qtype == dns.TypeAAAA && !resolver.IsUpstreamIPv6Supported(firstUp) {
		resp := new(dns.Msg)
		resp.SetReply(r)
		resp.Rcode = dns.RcodeNotImplemented
		w.WriteMsg(resp)
		return
	}

	// 迭代追踪 CNAME
	const maxCname = 10
	currentName := origName
	visited := make(map[string]bool)
	var addressChain []string
	var upstreamResp *dns.Msg

	for i := 0; i < maxCname; i++ {
		// 构造查询
		query := new(dns.Msg)
		query.SetQuestion(currentName, qtype)
		query.Id = r.Id
		query.RecursionDesired = true
		// 保留 EDNS0
		for _, extra := range r.Extra {
			if opt, ok := extra.(*dns.OPT); ok {
				query.Extra = append(query.Extra, opt)
			}
		}

		// 选上游
		up := resolver.GetUpstreamServer(currentName)
		proto, addr := resolver.ParseUpstreamServer(up)
		addressChain = append(addressChain, addr)

		// 发送并处理 TC
		upstreamResp, err = sendRequestWithRetry(proto, query, addr)
		if err != nil || upstreamResp == nil {
			fail := new(dns.Msg)
			fail.SetRcode(r, dns.RcodeServerFailure)
			w.WriteMsg(fail)
			return
		}

		// 非 NOERROR（包括 NXDOMAIN、NotImp、FormErr 等）直接返回，并做负向缓存
		if upstreamResp.Rcode != dns.RcodeSuccess {
			cacheNegative(currentName, qtype, upstreamResp)
			logResponse(srcIP, origName, addressChain, upstreamResp, start)
			upstreamResp.RecursionAvailable = true
			w.WriteMsg(upstreamResp)
			return
		}

		// 检查 Answer 里有没有 A/AAAA，或新的 CNAME
		foundA := false
		foundC := false
		for _, ans := range upstreamResp.Answer {
			if ans.Header().Rrtype == dns.TypeA || ans.Header().Rrtype == dns.TypeAAAA {
				foundA = true
			}
			if ans.Header().Rrtype == dns.TypeCNAME {
				c := ans.(*dns.CNAME).Target
				if !visited[c] {
					visited[c] = true
					currentName = c
					foundC = true
				}
			}
		}
		if foundA || !foundC {
			break
		}
	}

	// 最终结果
	if upstreamResp == nil {
		fail := new(dns.Msg)
		fail.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(fail)
		return
	}
	upstreamResp.Question = r.Question
	// 正向缓存
	if len(upstreamResp.Answer) > 0 {
		cachePositive(origName, qtype, upstreamResp)
	}

	// 打印日志
	logResponse(srcIP, origName, addressChain, upstreamResp, start)

	upstreamResp.RecursionAvailable = true
	w.WriteMsg(upstreamResp)
}

// sendRequestWithRetry 支持 UDP/DoT/DoH，UDP 且 TC=1时自动 TCP 重试
func sendRequestWithRetry(proto string, msg *dns.Msg, addr string) (*dns.Msg, error) {
	var resp *dns.Msg
	var err error

	switch proto {
	case "udp":
		resp, err = protocol.UdpRequest(msg, addr)
		if err == nil && resp.Truncated {
			c := &dns.Client{Net: "tcp"}
			resp, _, err = c.Exchange(msg, addr)
		}
	case "tls":
		resp, err = protocol.DoTRequest(msg, addr)
	case "https":
		resp, err = protocol.DoHRequest(msg, addr)
	default:
		resp = new(dns.Msg)
		resp.SetRcodeFormatError(msg)
	}

	return resp, err
}

// 正向缓存 (RFC 1035)
func cachePositive(name string, qtype uint16, msg *dns.Msg) {
	var minTTL uint32
	for _, rr := range msg.Answer {
		if minTTL == 0 || rr.Header().Ttl < minTTL {
			minTTL = rr.Header().Ttl
		}
	}
	if minTTL == 0 {
		return
	}
	shared.DnsCache.Set(utils.GenerateCacheKey(name, qtype), msg.Copy(), time.Duration(minTTL)*time.Second)
}

// 负向缓存 (RFC 2308)
func cacheNegative(name string, qtype uint16, msg *dns.Msg) {
	var negTTL uint32
	for _, ns := range msg.Ns {
		if soa, ok := ns.(*dns.SOA); ok {
			negTTL = soa.Minttl
			break
		}
	}
	if negTTL == 0 {
		return
	}
	shared.DnsCache.Set(utils.GenerateCacheKey(name, qtype), msg.Copy(), time.Duration(negTTL)*time.Second)
}

// 一行日志：源IP、查询、上游链、Rcode、Answer
func logResponse(srcIP, query string, chain []string, resp *dns.Msg, start time.Time)  {
	 elapsed := time.Since(start)
	var answers []string
	for _, rr := range resp.Answer {
		answers = append(answers, rr.String())
	}
	logger.GetLogger().InfoUpstream(
    srcIP,                      // 源 IP
    query,                      // 查询名
    strings.Join(chain, "->"),  // 上游链（以 "->" 连接）
    resp.Rcode,                 // 响应码（int）
    elapsed,                    // 耗时（time.Duration）
    answers,                    // []string 回答列表
)
}