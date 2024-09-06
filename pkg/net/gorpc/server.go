package gorpc

import (
	"fmt"
	"github.com/codingpoeta/go-demo/common"
	"github.com/codingpoeta/go-demo/utils"
	"github.com/valyala/gorpc"
	"log"
)

type Server struct {
	port    int
	ip      string
	dataGen common.DataGen
	s       *gorpc.Server
}

func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.ip, s.port)
}

func (s *Server) handle(clientAddr string, request interface{}) interface{} {
	req := request.(common.Request)
	res := common.Response{
		Body: s.dataGen.Get(fmt.Sprintf("key%d", req.CMD)),
	}
	res.Size = uint32(len(res.Body))
	return res
}

func (s *Server) Serve() error {
	s.s = &gorpc.Server{
		Addr:    s.Addr(),
		Handler: s.handle,
	}
	if err := s.s.Serve(); err != nil {
		log.Fatalf("Cannot start rpc server: %s", err)
	}
	return nil
}

func (s *Server) Close() {
	s.s.Stop()
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
	gorpc.RegisterType(common.Request{})
	gorpc.RegisterType(common.Response{})
	return svr, nil
}
