package gorpc

import (
	"github.com/codingpoeta/net-model-bench/common"
	"github.com/valyala/gorpc"
	"sync"
)

type Client struct {
	sync.Mutex
	addr      string
	gorpcClis chan *gorpc.Client
}

func NewClient(addr string, cons int) *Client {
	gorpc.RegisterType(common.Request{})
	gorpc.RegisterType(common.Response{})
	return &Client{
		addr:      addr,
		gorpcClis: make(chan *gorpc.Client, cons),
	}
}

func (c *Client) getGorpcClient() (*gorpc.Client, error) {
	var cli *gorpc.Client
	select {
	case cli = <-c.gorpcClis:
	default:
		cli = &gorpc.Client{
			Addr: c.addr,
		}
		cli.Start()
	}
	return cli, nil
}

func (c *Client) runWithClient(fn func(cli *gorpc.Client) error) error {
	cli, err := c.getGorpcClient()
	if err != nil {
		return err
	}
	err = fn(cli)
	if err != nil {
		return err
	}
	select {
	case c.gorpcClis <- cli:
	default:
		cli.Stop()
	}
	return nil
}

func (c *Client) Get(req common.Request) (*common.Response, error) {
	var resp common.Response
	err := c.runWithClient(func(cli *gorpc.Client) error {
		respI, err := cli.Call(req)
		if err != nil {
			return err
		}
		resp = respI.(common.Response)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Close() {
	close(c.gorpcClis)
}
