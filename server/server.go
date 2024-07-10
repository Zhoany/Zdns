package server

import (
	"NEWzDNS/cache"
	"NEWzDNS/config"
	"NEWzDNS/forward"
	"NEWzDNS/log"
	"NEWzDNS/pool"
	"NEWzDNS/rule"
	"github.com/gin-gonic/gin"
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"io"
	"net"
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
		err := w.WriteMsg(msg)
		if err != nil {
			return
		}
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
			clientIP, _, err := net.SplitHostPort(w.RemoteAddr().String())
			if err != nil {
				clientIP = "unknown"
			}
			log.BlockLogger.Info(
				"client_ip:", clientIP,
				" domain:", q.Name,
				" STATUS:", "BLOCKED",
			)
			msg.SetRcode(r, dns.RcodeNameError)
			err = w.WriteMsg(&msg)
			if err != nil {
				return
			}
			return
		}

		cacheKey := q.Name + "-" + dns.TypeToString[q.Qtype]
		cached, found := dnsCache.Get(cacheKey)
		if found {
			if responseMsg, ok := cached.(*dns.Msg); ok {
				log.RequestInfo(w, cacheKey, responseMsg, "cache")
				responseMsg.SetReply(r)
				err := w.WriteMsg(responseMsg)
				if err != nil {
					return
				}
				return
			}
		}

		upstream, _, found := rule.MatchDomain(q.Name)
		if !found {
			upstream = config.Cfg.CommonUpstream
		}

		var response *dns.Msg
		var err error

		switch q.Qtype {
		case dns.TypeA:
			response, err = forwardDNSRequest(q, upstream, r.Id)
			if err == nil && response != nil {
				for _, answer := range response.Answer {
					if answer.Header().Rrtype == dns.TypeA {
						dnsCache.Set(cacheKey, response)
						break
					}
				}
				log.RequestInfo(w, cacheKey, response, upstream.Address)
				response.SetReply(r)
				err = w.WriteMsg(response)
				if err != nil {
					return
				}
			}

		case dns.TypeAAAA:
			if config.Cfg.Server.ResolveIPv6 {
				response, err = forwardDNSRequest(q, upstream, r.Id)
				if err == nil && response != nil {
					for _, answer := range response.Answer {
						if answer.Header().Rrtype == dns.TypeAAAA {
							dnsCache.Set(cacheKey, response)

							break
						}
					}
					log.RequestInfo(w, cacheKey, response, upstream.Address)

					response.SetReply(r)
					err = w.WriteMsg(response)
					if err != nil {
						return
					}
				}
			}

		case dns.TypeSOA:
			response, err = forwardDNSRequest(q, upstream, r.Id)
			if err == nil && response != nil {
				for _, answer := range response.Answer {
					if answer.Header().Rrtype == dns.TypeSOA {
						dnsCache.Set(cacheKey, response)

						break
					}
				}
				log.RequestInfo(w, cacheKey, response, upstream.Address)

				response.SetReply(r)
				err = w.WriteMsg(response)
				if err != nil {
					return
				}
			}

		case dns.TypeMX:
			response, err = forwardDNSRequest(q, upstream, r.Id)
			if err == nil && response != nil {
				for _, answer := range response.Answer {
					if answer.Header().Rrtype == dns.TypeMX {
						dnsCache.Set(cacheKey, response)

						break
					}
				}
				log.RequestInfo(w, cacheKey, response, upstream.Address)

				response.SetReply(r)
				err = w.WriteMsg(response)
				if err != nil {
					return
				}
			}

		case dns.TypeNS:
			response, err = forwardDNSRequest(q, upstream, r.Id)
			if err == nil && response != nil {
				for _, answer := range response.Answer {
					if answer.Header().Rrtype == dns.TypeNS {
						dnsCache.Set(cacheKey, response)

						break
					}
				}
				log.RequestInfo(w, cacheKey, response, upstream.Address)

				response.SetReply(r)
				err = w.WriteMsg(response)
				if err != nil {
					return
				}
			}

		case dns.TypePTR:
			response, err = forwardDNSRequest(q, upstream, r.Id)
			if err == nil && response != nil {
				for _, answer := range response.Answer {
					if answer.Header().Rrtype == dns.TypePTR {
						dnsCache.Set(cacheKey, response)

						break
					}
				}
				log.RequestInfo(w, cacheKey, response, upstream.Address)

				response.SetReply(r)
				err = w.WriteMsg(response)
				if err != nil {
					return
				}
			}

		case dns.TypeCNAME:
			response, err = forwardDNSRequest(q, upstream, r.Id)
			if err == nil && response != nil {
				for _, answer := range response.Answer {
					if answer.Header().Rrtype == dns.TypeCNAME {
						dnsCache.Set(cacheKey, response)

						break
					}
				}
				log.RequestInfo(w, cacheKey, response, upstream.Address)

				response.SetReply(r)
				err = w.WriteMsg(response)
				if err != nil {
					return
				}
			}

		default:
			msg.SetRcode(r, dns.RcodeServerFailure)
			err = w.WriteMsg(&msg)
			if err != nil {
				return
			}
		}
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

	switch upstream.Protocol {
	case "DoH":
		response, err = forward.DoHRequest(msg, upstream.Address)
	case "UDP":
		client := new(dns.Client)
		response, _, err = client.Exchange(msg, upstream.Address)
	}
	if err != nil {
		return nil, err
	}
	return response, nil
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
