package protocol

import (
	"ZZDNS/logger"
	"fmt"
	_"log"

	"github.com/miekg/dns"
)

func UdpRequest(r *dns.Msg, upstream string) (*dns.Msg, error) {
	r.RecursionDesired = true
	client := new(dns.Client)
	resp, _, err := client.Exchange(r, upstream)
	if err != nil {
		wrappedErr := fmt.Errorf("UDP FAILED: %w", err)
		   logger.GetLogger().ErrorLog(
        "0.0.0.0",
        r.Question[0].Name,
        "UPSTREAM",
        wrappedErr,
    )
		return nil, err
	}
	return resp, nil
}
