package tcpsendfile

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/codingpoeta/go-demo/common"
)

type request struct {
	common.Request
	Buf [256]byte
}

func (r *request) Write(w io.Writer) error {
	buf := r.Buf[:]
	buf[0] = r.CMD & 0xF
	buf[1] = byte(len(r.Key))
	if len(r.Key) > 255 {
		buf[0] += byte(len(r.Key)>>8) << 4
	}
	copy(buf[2:], r.Key)
	if _, err := w.Write(buf[:len(r.Key)+2]); err != nil {
		return err
	}
	return nil
}

func (r *request) Read(conn net.Conn) error {
	if _, err := io.ReadFull(conn, r.Buf[:2]); err != nil {
		return err
	}
	r.CMD = r.Buf[0] & 0x0F
	size := int(r.Buf[1]) + int(r.Buf[0]>>4)<<8
	buf := r.Buf[:size]
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}
	r.Key = string(buf)
	return nil
}

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
		req := request{Request: req_}
		// fmt.Println("CMD:", req.CMD, "Key:", req.Key)
		if err := req.Write(conn); err != nil {
			fmt.Println("write error:", err)
			return err
		}
		n, err := conn.Read(res.Body)
		if err != nil {
			fmt.Println("read error:", err)
			return err
		}
		res.Size = uint32(n)
		res.Body = res.Body[:n]
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
