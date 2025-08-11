package protocol

import (
	"fmt"

	"ZZDNS/pool"
	"github.com/miekg/dns"
)

var dotPool = pool.NewPool()

func DoTRequest(msg *dns.Msg, upstream string) (*dns.Msg, error) {
	conn, err := dotPool.GetDotConn(upstream)
	if err != nil {
		return nil, fmt.Errorf("DOT POOL FAILED: %v", err)
	}
	defer dotPool.PutDotConn(conn)

	client := &dns.Conn{Conn: conn}
	err = client.WriteMsg(msg)
	if err != nil {
		return nil, fmt.Errorf("DOT WRITE FAILED: %v", err)
	}

	resp, err := client.ReadMsg()
	if err != nil {
		return nil, fmt.Errorf("DOT READ FAILED: %v", err)
	}

	return resp, nil
}
