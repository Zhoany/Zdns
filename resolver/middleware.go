package resolver

import "github.com/miekg/dns"

type Middleware func(dns.Handler) dns.Handler

func Chain(middlewares ...Middleware) Middleware {
	return func(final dns.Handler) dns.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}
