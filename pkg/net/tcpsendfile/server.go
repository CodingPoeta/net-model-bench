package tcpsendfile

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"github.com/codingpoeta/go-demo/common"
	"github.com/codingpoeta/go-demo/utils"
)

type Server struct {
	sync.Mutex
	listener net.Listener
	ip       string
	port     int
	dataGen  common.DataGen
}

func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.ip, s.port)
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
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			fmt.Println(err)
			break
		}
		go s.handle(conn)
	}
	return err
}

const basepath = "./data/file"

func (s *Server) handle(conn net.Conn) {
	tcpConn := conn.(*net.TCPConn)
	defer tcpConn.Close()
	for {
		var req request
		if err := req.Read(tcpConn); err != nil {
			log.Println(err)
			break
		}
		path := fmt.Sprintf("%s%d", basepath, req.CMD)
		file, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err != nil {
			log.Println(err)
			break
		}
		_, err = tcpConn.ReadFrom(file)
		file.Close()
		if err != nil {
			log.Println(err)
			break
		}
	}
}

func (s *Server) Close() {
}

func NewServer(ip, iname string, dg common.DataGen) (common.BlockServer, error) {
	for i := 0; i < 5; i++ {
		file, err := os.Create(fmt.Sprintf("%s%d", basepath, i))
		if err != nil {
			return nil, err
		}
		file.Write(dg.Get(fmt.Sprintf("key%d", i)))
		file.Close()
	}

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
