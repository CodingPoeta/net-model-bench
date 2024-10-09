package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/codingpoeta/go-demo/common"
	pb "github.com/codingpoeta/go-demo/pkg/net/grpc/proto"
	"github.com/codingpoeta/go-demo/utils"
	"google.golang.org/grpc"
)

type Server struct {
	port     int
	ip       string
	listener net.Listener
	dataGen  common.DataGen
	pb.UnimplementedBlockTransferServiceServer
}

func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.ip, s.port)
}

func (s *Server) Get(ctx context.Context, in *pb.BlockTransferRequest) (*pb.BlockTransferResponse, error) {
	//fmt.Println("Get: ", in.Key)
	res := &pb.BlockTransferResponse{}
	var l_buf []byte
	switch in.CMD {
	case 0:
		l_buf = s.dataGen.Get("key0")
	case 1:
		l_buf = s.dataGen.Get("key1")
	case 2:
		l_buf = s.dataGen.Get("key2")
	case 3:
		l_buf = s.dataGen.Get("key3")
	case 4:
		l_buf = s.dataGen.Get("key4")
	default:
		return nil, fmt.Errorf("invalid command")
	}
	res.Size = uint32(len(l_buf))
	res.Body = l_buf
	return res, nil
}

func (s *Server) Serve() (err error) {
	for {
		s.listener, err = net.Listen("tcp", s.Addr())
		if err == nil {
			break
		}
		s.port++
	}
	fmt.Println("listening on", s.Addr())
	svr := grpc.NewServer()
	pb.RegisterBlockTransferServiceServer(svr, s)

	err = svr.Serve(s.listener)
	if err != nil {
		fmt.Printf("failed to serve: %v", err)
		return
	}
	return err
}

func (s *Server) Close() {
}

func NewServer(ip, iname string, dg common.DataGen) (common.BlockServer, error) {
	ip, err := utils.FindLocalIP(ip, iname)
	if err != nil {
		return nil, err
	}

	svr := &Server{
		ip:      ip,
		port:    8000,
		dataGen: dg,
	}

	return svr, nil
}
