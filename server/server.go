package server

import (
	"NEWzDNS/cache"
	"NEWzDNS/config"
	"NEWzDNS/log"
	"NEWzDNS/pool"
	"NEWzDNS/rule"
	"encoding/base64"

	"net"

	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/miekg/dns"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

var dnsCache *cache.Cache

// StartDNSServer initializes and starts the DNS server
func StartDNSServer(sem chan struct{}) {
	dnsCache = cache.NewCache(int64(config.Cfg.Server.CacheSize))
	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleDNSRequestWrapper(w, r, sem)
	}) // Wraps handleDNSRequest
	server := &dns.Server{Addr: config.Cfg.Server.Address, Net: "udp"}
	if err := server.ListenAndServe(); err != nil {
		if config.Cfg.Server.EnableLogging && log.ErrorLogger != nil {
			log.ErrorLogger.Fatal("Failed to start DNS server", zap.Error(err))
		}
	}
}
func handleDNSRequestWrapper(w dns.ResponseWriter, r *dns.Msg, sem chan struct{}) {
	select {
	case sem <- struct{}{}: // Try to send a signal to the channel
		defer func() { <-sem }() // Read the signal from the channel after handling to free up a slot
		pool.SubmitToAnts(func() {

			handleDNSRequest(w, r)
		})
	default:
		// If the maximum concurrency is exceeded, return an error
		if config.Cfg.Server.EnableLogging && log.ErrorLogger != nil {
			log.ErrorLogger.Warn("Too many concurrent requests")
		}
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(msg)
	}
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	if r == nil {
		return
	}

	msg := dns.Msg{}
	msg.SetReply(r)
	msg.Authoritative = true

	for _, q := range r.Question {
		blocked := rule.IsBlocked(q.Name)
		if blocked {
			msg.SetRcode(r, dns.RcodeNameError)
			w.WriteMsg(&msg)
			return
		}

		cached, found := dnsCache.Get(q.Name)
		if found {
			if responseMsg, ok := cached.(*dns.Msg); ok {
				logRequestInfo(w, q.Name, responseMsg, "cache")
				responseMsg.SetReply(r)
				w.WriteMsg(responseMsg)
				return
			}
		}

		upstream, _, found := rule.MatchDomain(q.Name)
		if !found {
			upstream = config.Cfg.CommonUpstream

		}

		var ipv4Response, ipv6Response *dns.Msg
		var ipv4Err, ipv6Err error

		// Forward DNS request for A record (IPv4)
		q4 := dns.Question{Name: q.Name, Qtype: dns.TypeA, Qclass: dns.ClassINET}
		ipv4Response, ipv4Err = forwardDNSRequest(q4, upstream, r.Id)

		// Forward DNS request for AAAA record (IPv6) if resolve_ipv6 is true
		if config.Cfg.Server.ResolveIPv6 {
			q6 := dns.Question{Name: q.Name, Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}
			ipv6Response, ipv6Err = forwardDNSRequest(q6, upstream, r.Id)
		}

		// Merge the results
		if ipv4Err != nil && (!config.Cfg.Server.ResolveIPv6 || ipv6Err != nil) {
			msg.SetRcode(r, dns.RcodeServerFailure)
			w.WriteMsg(&msg)
			return
		}

		// If both IPv4 and IPv6 responses are present, do not cache
		if ipv4Response != nil && ipv6Response != nil {
			ipv4Response.Answer = append(ipv4Response.Answer, ipv6Response.Answer...)
			logRequestInfo(w, q.Name, ipv4Response, upstream.Address)
		} else if ipv4Response != nil {
			// Cache only IPv4 response
			for _, answer := range ipv4Response.Answer {
				if answer.Header().Rrtype == dns.TypeA {
					dnsCache.Set(q.Name, ipv4Response)
					break
				}
			}
			logRequestInfo(w, q.Name, ipv4Response, upstream.Address)
		} else if ipv6Response != nil {
			logRequestInfo(w, q.Name, ipv6Response, upstream.Address)
		}

		if ipv4Response != nil {
			ipv4Response.SetReply(r)
			w.WriteMsg(ipv4Response)
		} else if ipv6Response != nil {
			ipv6Response.SetReply(r)
			w.WriteMsg(ipv6Response)
		}
	}
}

func logRequestInfo(w dns.ResponseWriter, domain string, response *dns.Msg, upstream string) {
	if !config.Cfg.Server.EnableLogging {
		return
	}

	clientIP, _, err := net.SplitHostPort(w.RemoteAddr().String())
	if err != nil {
		clientIP = "unknown"
	}
	var resolvedResults strings.Builder

	for _, answer := range response.Answer {
		switch a := answer.(type) {
		case *dns.A:
			if resolvedResults.Len() > 0 {
				resolvedResults.WriteString(", ")
			}
			resolvedResults.WriteString(a.A.String())
		case *dns.AAAA:
			if resolvedResults.Len() > 0 {
				resolvedResults.WriteString(", ")
			}
			resolvedResults.WriteString(a.AAAA.String())
		}
	}

	if resolvedResults.Len() > 0 {
		log.RequestLogger.Info(
			"client_ip:", clientIP,
			" domain:", domain,
			" resolved_results:", resolvedResults.String(),
			" upstream:", upstream,
		)
	}
}

// forwardDNSRequest forwards the DNS request to the upstream server
func forwardDNSRequest(q dns.Question, upstream config.Upstream, id uint16) (*dns.Msg, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(q.Name, q.Qtype)
	msg.RecursionDesired = true
	msg.Id = id

	var response *dns.Msg
	var err error

	address := appendPort(upstream.Address, upstream.Port)

	if upstream.Protocol == "DoH" {
		response, err = forwardDoHRequest(msg, address)
	} else {
		client := new(dns.Client)
		response, _, err = client.Exchange(msg, address)
	}

	if err != nil {
		return nil, err
	}
	return response, nil
}

// forwardDoHRequest forwards the DNS over HTTPS request
func forwardDoHRequest(msg *dns.Msg, upstream string) (*dns.Msg, error) {
	dnsRequest, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	encodedRequest := base64.RawURLEncoding.EncodeToString(dnsRequest)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(upstream + "?dns=" + encodedRequest)
	req.Header.SetMethod(fasthttp.MethodGet)
	req.Header.Set("Accept", "application/dns-message")

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	client := pool.GetClient()
	defer pool.ReturnClient(client)

	err = client.DoTimeout(req, resp, 5*time.Second)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, err
	}

	body := resp.Body()
	dnsResponse := new(dns.Msg)
	if err := dnsResponse.Unpack(body); err != nil {
		return nil, err
	}

	return dnsResponse, nil
}

// appendPort appends the port to the address if it is not already included
func appendPort(address, port string) string {
	if !strings.Contains(address, ":") {
		return address + ":" + port
	}
	return address
}

// StartAdminServer initializes and starts the admin server
func StartAdminServer(addr string) {
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard

	router := gin.New()
	router.GET("/", adminRequestHandler)

	if err := router.Run(addr); err != nil {
		if config.Cfg.Server.EnableLogging && log.ErrorLogger != nil {
			log.ErrorLogger.Fatal("Failed to start admin server", zap.Error(err))
		}
	}
}

// adminRequestHandler handles requests to the admin server
func adminRequestHandler(c *gin.Context) {
	c.String(200, "Admin server is running")
}
