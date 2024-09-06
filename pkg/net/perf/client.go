package perf

import (
	"fmt"
	"github.com/codingpoeta/go-demo/common"
	"net"
	"sync"
	"time"
)

type Client struct {
	sync.Mutex
	addr  string
	conns chan net.Conn
}

func (c *Client) getConn() (net.Conn, error) {
	var conn net.Conn
	select {
	case conn = <-c.conns:
	default:
		// one connect at a time
		c.Lock()
		defer c.Unlock()
		dialer := &net.Dialer{Timeout: time.Second + time.Millisecond*100, KeepAlive: time.Minute}
		c, err := dialer.Dial("tcp", c.addr)
		if err != nil {
			return nil, err
		}
		conn = c
	}
	return conn, nil
}

func (c *Client) withConn(f func(conn net.Conn) error) error {
	conn, err := c.getConn()
	if err != nil {
		return err
	}
	err = f(conn)
	if err != nil {
		return err
	}
	select {
	case c.conns <- conn:
	default:
		_ = conn.Close()
	}
	return nil
}

func (c *Client) Close() {
	close(c.conns)
}

func (c *Client) Get(req_ common.Request) (*common.Response, error) {
	var res common.Response
	res.Body = make([]byte, 4<<20)

	err := c.withConn(func(conn net.Conn) error {
		var got, cnt int
		for got < 4<<20 {
			n, err := conn.Read(res.Body[got:])
			if err != nil {
				fmt.Println("read error:", err)
				return err
			}
			got += n
			cnt++
		}
		if cnt > 0 {
			fmt.Println("read count:", cnt)
		}
		res.Size = uint32(got)
		res.Body = res.Body[:got]
		return nil
	})
	return &res, err
}

func NewClient(addr string, cons int) common.BlockClient {
	return &Client{
		addr:  addr,
		conns: make(chan net.Conn, cons),
	}
}
