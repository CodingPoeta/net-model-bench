package iorpc

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/codingpoeta/net-model-bench/common"
	"github.com/hexilee/iorpc"
)

type Client struct {
	addr string
	cli  *iorpc.Client
}

func NewClient(addr string, conns int) *Client {
	NewDispatcherForClient()
	c := iorpc.NewTCPClient(addr)
	c.DisableCompression = true
	c.Conns = conns
	c.CloseBody = true
	c.Start()
	return &Client{
		addr: addr,
		cli:  c,
	}
}

func (c *Client) Close() {
	c.cli.Stop()
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

	req := iorpc.Request{
		Service: ServiceReadData,
		Headers: &ReadHeaders{
			CMD: uint64(req_.CMD),
		},
	}
	resp, err := c.cli.CallTimeout(req, time.Hour)
	if err != nil {
		fmt.Printf("call error: %v\n", err)
		return nil, err
	}
	_, err = io.ReadFull(resp.Body.Reader, res.Body[:resp.Body.Size])
	if err != nil {
		fmt.Printf("read error: %v\n", err)
		return nil, err
	}

	return &common.Response{
		Body: res.Body,
		Size: uint32(len(res.Body)),
		BB:   res.BB,
	}, err
}
