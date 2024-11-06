package tcpsendfile

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/codingpoeta/net-model-bench/common"
	"github.com/codingpoeta/net-model-bench/pkg/iorpc"
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
	dg    common.DataGen
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

var staticBuf = make([]byte, 5120*1024)

var payloadBufPool = &sync.Pool{
	New: func() any {
		return &common.BodyBuffer{
			Buf: make([]byte, 5120*1024),
		}
	},
}

func (c *Client) Get(req_ common.Request) (*common.Response, error) {
	var res common.Response
	payloadBuf := payloadBufPool.Get().(*common.BodyBuffer)
	payloadBuf.Release = func() {
		payloadBufPool.Put(payloadBuf)
	}
	res.Body = payloadBuf.Buf
	res.BB = payloadBuf
	res.BB.Inc()

	err := c.withConn(func(conn net.Conn) (err error) {
		req := request{Request: req_}
		// fmt.Println("CMD:", req.CMD, "Key:", req.Key)
		if err = req.Write(conn); err != nil {
			fmt.Println("write error:", err)
			return err
		}
		var n int
		r, ok := conn.(iorpc.IsConn)
		if ok {
			n = int(c.dg.GetSize(fmt.Sprintf("%s%d", "key", req.CMD)))
			_, err_ := iorpc.PipeConn(r, n)
			err = err_
			// defer p.Close()
			res.Body = staticBuf[:n]
		}
		if !ok || err != nil {
			n, err = io.ReadFull(conn, res.Body[:c.dg.GetSize(fmt.Sprintf("%s%d", "key", req.CMD))])
		}
		if err != nil {
			fmt.Println("read error:", err)
			return err
		}
		res.Size = uint32(n)
		res.Body = res.Body[:n]
		return nil
	})
	return &common.Response{
		Body: res.Body,
		Size: uint32(len(res.Body)),
		BB:   res.BB,
	}, err
}

func NewClient(addr string, cons int, datagen common.DataGen) common.BlockClient {
	return &Client{
		dg:    datagen,
		addr:  addr,
		conns: make(chan net.Conn, cons),
	}
}
