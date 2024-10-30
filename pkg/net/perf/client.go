package perf

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/codingpoeta/net-model-bench/common"
)

type Client struct {
	sync.Mutex
	addr  string
	conns []net.Conn
}

func (c *Client) getConn() (net.Conn, error) {
	dialer := &net.Dialer{Timeout: time.Second + time.Millisecond*100, KeepAlive: time.Minute}
	conn, err := dialer.Dial("tcp", c.addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *Client) Close() {
	for _, conn := range c.conns {
		_ = conn.Close()
	}
}

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

	var err error

	id, _ := strconv.Atoi(req_.Key)
	conn := c.conns[id%len(c.conns)]
	if conn == nil {
		conn, err = c.getConn()
		if err != nil {
			return nil, err
		}
		c.conns[id%len(c.conns)] = conn
	}

	var got, cnt, n int
	for got < 4<<20 {
		n, err = conn.Read(res.Body[got:])
		if err != nil {
			fmt.Println("read error:", err)
			return nil, err
		}
		got += n
		cnt++
	}
	if cnt > 0 {
		// fmt.Println("read count:", cnt)
	}
	res.Size = uint32(got)
	res.Body = res.Body[:got]

	return &common.Response{
		Body: res.Body,
		Size: uint32(len(res.Body)),
		BB:   res.BB,
	}, err
}

func NewClient(addr string, cons int) common.BlockClient {
	return &Client{
		addr:  addr,
		conns: make([]net.Conn, cons),
	}
}
