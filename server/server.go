package server

import (
	"NEWzDNS/cache"
	"NEWzDNS/config"
	"NEWzDNS/log"
	"NEWzDNS/pool"
	"NEWzDNS/rule"
	"encoding/base64"
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

// handleDNSRequestWrapper wraps handleDNSRequest to limit the number of concurrent connections
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

// handleDNSRequest processes the DNS request
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
			if config.Cfg.Server.EnableLogging && log.RequestLogger != nil {
				log.RequestLogger.Warn("Blocked query for domain", zap.String("domain", q.Name))
			}
			msg.SetRcode(r, dns.RcodeNameError)
			w.WriteMsg(&msg)
			return
		}

		cached, found := dnsCache.Get(q.Name)
		if found {
			if responseMsg, ok := cached.(*dns.Msg); ok {
				if config.Cfg.Server.EnableLogging && log.RequestLogger != nil {
					log.RequestLogger.Info("Cache hit for domain", zap.String("domain", q.Name))
				}
				responseMsg.SetReply(r)
				w.WriteMsg(responseMsg)
				logRequestAndResponse(r, responseMsg)
				return
			}
		}

		upstream, _, found := rule.MatchDomain(q.Name)
		if found {
			if config.Cfg.Server.EnableLogging && log.RequestLogger != nil {
				log.RequestLogger.Info("Matched rule for domain", zap.String("domain", q.Name), zap.String("upstream", upstream.Address))
			}
		} else {
			upstream = config.Cfg.CommonUpstream
			if config.Cfg.Server.EnableLogging && log.RequestLogger != nil {
				log.RequestLogger.Info("Using common upstream for domain", zap.String("domain", q.Name), zap.String("upstream", upstream.Address))
			}
		}

		response, err := forwardDNSRequest(q, upstream, r.Id)
		if err != nil {
			if config.Cfg.Server.EnableLogging && log.ErrorLogger != nil {
				log.ErrorLogger.Error("Failed to forward DNS request", zap.String("domain", q.Name), zap.Error(err))
			}
			msg.SetRcode(r, dns.RcodeServerFailure)
			w.WriteMsg(&msg)
			return
		}

		if config.Cfg.Server.EnableLogging && log.RequestLogger != nil {
			log.RequestLogger.Info("Forwarded DNS request", zap.String("domain", q.Name), zap.String("upstream", upstream.Address))
		}

		// Only store IPv4 or IPv6
		for _, answer := range response.Answer {
			if answer.Header().Rrtype == dns.TypeAAAA || answer.Header().Rrtype == dns.TypeA {
				dnsCache.Set(q.Name, response)
				break
			}
		}

		response.SetReply(r)
		w.WriteMsg(response)
		logRequestAndResponse(r, response)
	}
}

// logRequestAndResponse logs the DNS request and response
func logRequestAndResponse(request *dns.Msg, response *dns.Msg) {
	if config.Cfg.Server.EnableLogging && log.RequestLogger != nil {
		log.RequestLogger.Info("DNS Request",
			zap.String("request", request.String()),
			zap.String("response", response.String()))
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
