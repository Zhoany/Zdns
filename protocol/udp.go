package protocol

import (
	"log"

	"github.com/miekg/dns"
)

func UdpRequest(r *dns.Msg, upstream string) (*dns.Msg, error) {
	r.RecursionDesired = true
	client := new(dns.Client)
	resp, _, err := client.Exchange(r, upstream)
	if err != nil {
		log.Printf("Failed to get response from UDP server: %v", err)
		return nil, err
	}
	return resp, nil
}
