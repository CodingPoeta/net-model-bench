package tcppool

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
	compressOn bool
	crcOn      bool
	Buf        [256]byte
}

func (r *request) Write(w io.Writer) error {
	buf := r.Buf[:]
	buf[0] = r.CMD & 0xF
	buf[1] = byte(len(r.Key))
	buf[2] = 0
	if r.compressOn {
		buf[2] |= 0x01
	}
	if r.crcOn {
		buf[2] |= 0x02
	}
	if len(r.Key) > 255 {
		buf[0] += byte(len(r.Key)>>8) << 4
	}
	copy(buf[3:], r.Key)
	if _, err := w.Write(buf[:len(r.Key)+3]); err != nil {
		return err
	}
	return nil
}

func (r *request) Read(conn net.Conn) error {
	if _, err := io.ReadFull(conn, r.Buf[:3]); err != nil {
		return err
	}
	r.CMD = r.Buf[0] & 0x0F
	size := int(r.Buf[1]) + int(r.Buf[0]>>4)<<8
	if r.Buf[2]&0x01 != 0 {
		r.compressOn = true
	}
	if r.Buf[2]&0x02 != 0 {
		r.crcOn = true
	}

	buf := r.Buf[:size]
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}
	r.Key = string(buf)
	return nil
}

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
		// bfsz := 10 << 20
		// c.(*net.TCPConn).SetWriteBuffer(bfsz)
		// c.(*net.TCPConn).SetReadBuffer(bfsz)
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
	var res = &response{}

	err := c.withConn(func(conn net.Conn) error {
		req := request{Request: req_, compressOn: c.compressOn, crcOn: c.crcOn}
		// fmt.Println("CMD:", req.CMD, "Key:", req.Key)
		for i := 0; i < req_.Batch; i++ {
			if err := req.Write(conn); err != nil {
				fmt.Println("write error:", err)
				return err
			}
		}
		for i := 0; i < req_.Batch; i++ {
			if err := res.Read(conn); err != nil {
				fmt.Println("read error:", err)
				return err
			} else if res.Err != nil {
				fmt.Println("response error:", res.Err)
				return res.Err
			}
		}
		return nil
	})
	return &common.Response{
		Body: res.Body,
		Size: uint32(len(res.Body)),
		BB:   res.BB,
	}, err
}
