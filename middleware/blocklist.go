package middleware

import (
	"strings"

	"ZZDNS/config"

	"github.com/miekg/dns"
)

// BlocklistMiddleware checks if the domain is in the blocklist and rejects the request if it is
func BlocklistMiddleware(next dns.Handler) dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		rdomain := strings.TrimSuffix(r.Question[0].Name, ".")

		// 使用哈希表进行匹配
		if _, blocked := config.Blocklist[rdomain]; blocked {
			m := new(dns.Msg)
			m.SetRcode(r, dns.RcodeRefused)
			w.WriteMsg(m)
			return
		}

		next.ServeDNS(w, r)
	})
}
