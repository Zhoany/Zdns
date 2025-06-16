package pool

import (
	"crypto/tls"
	"fmt"
	"io"
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

// isConnAlive checks if a connection is still usable
func isConnAlive(conn net.Conn) bool {
	if conn == nil {
		return false
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Millisecond)); err != nil {
		return false
	}
	var buf [1]byte
	if _, err := conn.Read(buf[:]); err != nil {
		if err == io.EOF {
			return false
		}
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			conn.SetReadDeadline(time.Time{})
			return true
		}
		return false
	}
	return true
}

// checkServerAlive checks if a server can be reached via TCP within timeout
func checkServerAlive(address string) bool {
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
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
	if conn := p.DotConnPool.Get(); conn != nil {
		c := conn.(net.Conn)
		if isConnAlive(c) {
			return c, nil
		}
		c.Close()
	}

	if !checkServerAlive(upstreamDot) {
		return nil, fmt.Errorf("server %s not reachable", upstreamDot)
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	newConn, err := tls.DialWithDialer(dialer, "tcp", upstreamDot, &tls.Config{})
	if err != nil {
		log.Printf("Failed to connect to DoT server: %v", err)
		return nil, err
	}
	return newConn, nil
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
