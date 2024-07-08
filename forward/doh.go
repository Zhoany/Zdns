package forward

import (
	"NEWzDNS/pool"
	"encoding/base64"
	"github.com/miekg/dns"
	"github.com/valyala/fasthttp"
	"time"
)

// DoHRequest forwards the DNS over HTTPS request
func DoHRequest(msg *dns.Msg, upstream string) (*dns.Msg, error) {
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
