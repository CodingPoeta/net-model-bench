package tcpsendfile

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"github.com/codingpoeta/net-model-bench/common"
	"github.com/codingpoeta/net-model-bench/utils"
)

type Server struct {
	sync.Mutex
	mode     common.ServerMode
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

func (s *Server) handle(conn net.Conn) {
	tcpConn := conn.(*net.TCPConn)
	defer tcpConn.Close()
	for {
		var req request
		if err := req.Read(tcpConn); err != nil {
			log.Println(err)
			break
		}
		var err error
		key := fmt.Sprintf("key%d", req.CMD)
		switch s.mode {
		case common.MODE_SENDBUF:
			buf := s.dataGen.Get(key)
			_, err = tcpConn.Write(buf)
		case common.MODE_SPLICE:
			reader := s.dataGen.GetReadCloser(key)
			err = utils.SpliceSendFile(tcpConn, reader.(*os.File), s.dataGen.GetSize(fmt.Sprintf("key%d", req.CMD)))
			reader.Close()
		default:
			reader := s.dataGen.GetReadCloser(key)
			_, err = tcpConn.ReadFrom(s.dataGen.GetReadCloser(key))
			reader.Close()
		}
		if err != nil {
			log.Println(err)
			break
		}
	}
}

func (s *Server) Close() {
}

func NewServer(ip, iname string, dg common.DataGen) (common.BlockServer, error) {
	ip, err := utils.FindLocalIP(ip, iname)
	if err != nil {
		return nil, err
	}

	svr := &Server{
		mode:    common.MODE_SENDFILE,
		ip:      ip,
		port:    8000,
		dataGen: dg,
	}
	mode := os.Getenv("SERVER_MODE")
	switch mode {
	case "sendbuf":
		svr.mode = common.MODE_SENDBUF
	case "splice":
		svr.mode = common.MODE_SPLICE
	default:
		svr.mode = common.MODE_SENDFILE
	}

	return svr, nil
}
