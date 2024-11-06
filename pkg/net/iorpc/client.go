package iorpc

import (
	"fmt"
	"sync"
	"sync/atomic"
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
	// c.CloseBody = true
	c.FlushDelay = time.Microsecond * 10
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

var staticBuf = make([]byte, 5120*1024)
var nextID atomic.Uint64

func (c *Client) Get(req_ common.Request) (*common.Response, error) {
	var res common.Response
	if nextID.Load() > 20 {
		nextID.Store(0)
	}

	req := iorpc.Request{
		Service: ServiceReadData,
		Headers: &ReadHeaders{
			CMD: uint64(req_.CMD),
			ID:  nextID.Add(1),
		},
	}
	// fmt.Println("----------reqID:  ", req.Headers.(*ReadHeaders).ID)
	resp, err := c.cli.CallTimeout(req, time.Hour)
	if err != nil {
		fmt.Printf("call error: %v\n", err)
		return nil, err
	}

	if _, ok := resp.Headers.(*ReadHeaders); ok {
		res.Size = uint32(resp.Headers.(*ReadHeaders).Size)
		// fmt.Println("++++++++++respID: ", resp.Headers.(*ReadHeaders).ID)
	} else {
		res.Size = uint32(resp.Body.Size)
	}

	resp.Body.Reader.Close()

	res.Body = staticBuf[:res.Size]
	return &res, err
}
