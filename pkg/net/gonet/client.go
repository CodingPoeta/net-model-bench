package gonet

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/codingpoeta/go-demo/common"
)

type Client struct {
	sync.Mutex
	addr       string
	compressOn bool
	crcOn      bool
	conns      chan net.Conn
}

func NewClient(addr string, cons int, compressOn, crcOn bool) common.BlockClient {
	return &Client{
		addr:       addr,
		compressOn: compressOn,
		crcOn:      crcOn,
		conns:      make(chan net.Conn, cons),
	}
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
	var res response

	err := c.withConn(func(conn net.Conn) error {
		req := request{Request: req_, compressOn: c.compressOn, crcOn: c.crcOn}
		// fmt.Println("CMD:", req.CMD, "Key:", req.Key)
		if err := req.Write(conn); err != nil {
			fmt.Println("write error:", err)
			return err
		}
		if err := res.Read(conn); err != nil {
			fmt.Println("read error:", err)
			return err
		} else if res.Err != nil {
			fmt.Println("response error:", res.Err)
			return res.Err
		}
		return nil
	})
	return &(res.Response), err
}
