package iorpc

import (
	"fmt"
	"github.com/codingpoeta/net-model-bench/common"
	"github.com/codingpoeta/net-model-bench/utils"
	"github.com/hexilee/iorpc"
	"time"
)

type Server struct {
	s *iorpc.Server

	dispatcher *iorpc.Dispatcher

	ip      string
	port    int
	dataGen common.DataGen
}

func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.ip, s.port)
}

func (s *Server) Serve() error {
	s.s = &iorpc.Server{
		FlushDelay: time.Microsecond * 10,
		Handler:    s.dispatcher.HandlerFunc(),
	}
	s.s.Addr = s.Addr()
	fmt.Println("listening on", s.Addr())

	return s.s.Serve()
}

func (s *Server) Close() {
	return
}

func NewServer(ip, iname string, dg common.DataGen) (common.BlockServer, error) {
	ip, err := utils.FindLocalIP(ip, iname)
	if err != nil {
		return nil, err
	}
	svr := &Server{
		ip:         ip,
		port:       8000,
		dispatcher: NewDispatcher(),
	}
	iorpc.RegisterHeaders(func() iorpc.Headers {
		return new(ReadHeaders)
	})
	addServiceNoop(svr.dispatcher)
	addServiceReadData(svr.dispatcher, dg)
	addServiceReadMemory(svr.dispatcher)
	return svr, nil
}
