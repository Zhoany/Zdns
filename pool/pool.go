package pool

import (
	"crypto/tls"

	"log"
	"net"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

type Pool struct {
	DotConnPool    sync.Pool
	FastHttpClient sync.Pool
}

func NewPool() *Pool {
	return &Pool{
		DotConnPool: sync.Pool{
			New: func() interface{} {
				return nil
			},
		},
		FastHttpClient: sync.Pool{
			New: func() interface{} {
				return &fasthttp.Client{
					ReadTimeout:  5 * time.Second,
					WriteTimeout: 5 * time.Second,
				}
			},
		},
	}
}

func (p *Pool) GetDotConn(upstreamDot string) (net.Conn, error) {
	conn := p.DotConnPool.Get()
	if conn == nil {
		dialer := &net.Dialer{Timeout: 5 * time.Second}
		newConn, err := tls.DialWithDialer(dialer, "tcp", upstreamDot, &tls.Config{})
		if err != nil {
			log.Printf("Failed to connect to DoT server: %v", err)
			return nil, err
		}
		return newConn, nil
	}
	return conn.(net.Conn), nil
}

func (p *Pool) PutDotConn(conn net.Conn) {
	if conn != nil {
		p.DotConnPool.Put(conn)
	}
}

func (p *Pool) GetHttpClient() *fasthttp.Client {
	client := p.FastHttpClient.Get().(*fasthttp.Client)
	return client
}

func (p *Pool) PutHttpClient(client *fasthttp.Client) {
	p.FastHttpClient.Put(client)
}
