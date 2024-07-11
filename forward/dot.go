package forward

import (
	"crypto/tls"
	"github.com/miekg/dns"
	"net"
	"time"
)

// DoTRequest forwards the DNS over TLS request
func DoTRequest(msg *dns.Msg, upstream string) (*dns.Msg, error) {
	dnsRequest, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	conn, err := tls.DialWithDialer(&net.Dialer{
		Timeout: 5 * time.Second,
	}, "tcp", upstream, &tls.Config{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Write the length-prefixed DNS request
	length := make([]byte, 2)
	length[0], length[1] = byte(len(dnsRequest)>>8), byte(len(dnsRequest)&0xFF)
	if _, err := conn.Write(append(length, dnsRequest...)); err != nil {
		return nil, err
	}

	// Read the length-prefixed DNS response
	length = make([]byte, 2)
	if _, err := conn.Read(length); err != nil {
		return nil, err
	}
	responseLength := int(length[0])<<8 | int(length[1])

	body := make([]byte, responseLength)
	if _, err := conn.Read(body); err != nil {
		return nil, err
	}

	dnsResponse := new(dns.Msg)
	if err := dnsResponse.Unpack(body); err != nil {
		return nil, err
	}

	return dnsResponse, nil
}
