package grpc

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codingpoeta/net-model-bench/common"
	pb "github.com/codingpoeta/net-model-bench/pkg/net/grpc/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var createClientMutex sync.Mutex

type grpcClient struct {
	refCnt atomic.Int32
	conn   *grpc.ClientConn
	cli    pb.BlockTransferServiceClient
}

func (g *grpcClient) Close() error {
	if g.refCnt.Add(-1) > 0 {
		return nil
	}
	return g.conn.Close()
}

type client struct {
	addr           string
	threadsPerCons int
	grpcClis       chan *grpcClient
}

func (c *client) getGrpcClient() (*grpcClient, error) {
retry:
	select {
	case grpcCli := <-c.grpcClis:
		return grpcCli, nil
	default:
		if createClientMutex.TryLock() {
			defer createClientMutex.Unlock()
		} else {
			goto retry
		}
		fmt.Println("new grpc client")
		conn, err := grpc.Dial(c.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}
		grpcCli := &grpcClient{
			conn: conn,
			cli:  pb.NewBlockTransferServiceClient(conn),
		}
		grpcCli.refCnt.Store(int32(c.threadsPerCons))
		for i := 1; i < c.threadsPerCons; i++ {
			c.grpcClis <- grpcCli
		}
		return grpcCli, nil
	}
}

func (c *client) runWithClient(fn func(cli *grpcClient) error) error {
	grpcCli, err := c.getGrpcClient()
	if err != nil {
		return err
	}
	err = fn(grpcCli)
	if err != nil {
		return err
	}

	select {
	case c.grpcClis <- grpcCli:
	default:
		_ = grpcCli.Close()
	}
	return nil
}

func (c *client) Get(req common.Request) (*common.Response, error) {
	var res common.Response
	err := c.runWithClient(func(cli *grpcClient) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		r, err := cli.cli.Get(ctx, &pb.BlockTransferRequest{
			Key: req.Key,
			CMD: uint32(req.CMD),
		})
		if err != nil {
			return err
		}
		res.Body = r.Body
		res.Size = r.Size
		return nil
	})
	return &res, err
}

func (c *client) Close() {
	close(c.grpcClis)
}

func NewClient(addr string, threadsPerCons, cons int) common.BlockClient {
	return &client{
		addr:           addr,
		threadsPerCons: threadsPerCons,
		grpcClis:       make(chan *grpcClient, cons*threadsPerCons),
	}
}
