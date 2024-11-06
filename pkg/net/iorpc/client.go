package iorpc

import (
	"fmt"
	"sync"
	"time"

	"github.com/codingpoeta/net-model-bench/common"
	"github.com/codingpoeta/net-model-bench/pkg/iorpc"
)

type Client struct {
	addr string
	cli  *iorpc.Client
}

func NewClient(addr string, conns int) *Client {
	NewDispatcherForClient()
	iorpc.RegisterHeaders(func() iorpc.Headers {
		return new(ReadHeaders)
	})
	c := iorpc.NewTCPClient(addr)
	c.DisableCompression = true
	c.Conns = conns
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

	if buf, ok := resp.Body.Reader.(iorpc.Buffer); ok {
		res.Body = buf.Bytes()
		res.Size = uint32(len(res.Body))
	}

	return &res, err
}
