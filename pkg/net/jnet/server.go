package jnet

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"sync"

	"github.com/codingpoeta/go-demo/common"
	"github.com/codingpoeta/go-demo/utils"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

var reqPool *sync.Pool = &sync.Pool{
	New: func() any {
		return &request{}
	},
}

type IOQueueBackend struct {
	conn    net.Conn
	dataGen common.DataGen
	respCH  chan *response
}

var workPool = NewWorkerPool()

func NewIOQueueBackend(dataGen common.DataGen, c net.Conn) *IOQueueBackend {
	q := &IOQueueBackend{
		conn:    c,
		dataGen: dataGen,
		respCH:  make(chan *response, 2048),
	}
	go q.submitWorker()
	go q.recvWorker()
	return q
}

func (q *IOQueueBackend) recvWorker() {
	fmt.Println("server recv worker started")
	defer fmt.Println("server recv worker closed")

	var desc batchHdrDesc
	var reqHeaderBuffer [1024 * 1024]byte
	reqs := make([]*request, 0)
	for {
		reqs = reqs[:0]
		_, err := io.ReadFull(q.conn, desc.Buf[:])
		if err != nil {
			// TODO:
			panic(err)
		}
		err = desc.Decode(desc.Buf[:])
		if err != nil {
			panic(err)
		}
		// read headers
		_, err = io.ReadFull(q.conn, reqHeaderBuffer[:desc.HeadLength])
		if err != nil {
			panic(err)
		}
		left := desc.HeadLength
		idx := 0
		for left > 0 {
			req := reqPool.Get().(*request)
			n, err := req.Decode(reqHeaderBuffer[idx:])
			if err != nil {
				panic(err)
			}
			left -= uint32(n)
			idx += n
			if req.ContentLen > 0 {
				buf := bytes.NewBuffer(nil)
				_, err = io.CopyN(buf, q.conn, int64(req.ContentLen))
				if err != nil {
					panic(err)
				}
				req.Body = buf
			}
			reqs = append(reqs, req)
		}
		for idx, req := range reqs {
			req.batchId = desc.Cookie
			req.idx = uint32(idx)
			req.backend = q
			workPool.Submit(req)
		}
	}
}

var respPool *sync.Pool = &sync.Pool{
	New: func() any {
		return &response{}
	},
}

func (q *IOQueueBackend) processRequest(req *request) {
	resp := respPool.Get().(*response)
	resp.Idx = req.idx
	resp.BatchId = req.batchId
	var buf []byte
	switch req.CMD {
	case 0:
		buf = q.dataGen.Get("key0")
	case 1:
		buf = q.dataGen.Get("key1")
	case 2:
		buf = q.dataGen.Get("key2")
	case 3:
		buf = q.dataGen.Get("key3")
	case 4:
		buf = q.dataGen.Get("key4")
	default:
		resp.ErrorCode = 1
		resp.ErrorMsg = "invalid command"
	}
	reqPool.Put(req)
	resp.ContentLen = uint32(len(buf))
	resp.Body = bytes.NewBuffer(buf)
	q.submit(resp)
}

func (q *IOQueueBackend) submit(resp *response) {
	resp.encodedHead = resp.Encode()
	q.respCH <- resp
}

func (q *IOQueueBackend) submitWorker() {
	// no batch in this version
	fmt.Println("server submit worker started")
	defer fmt.Println("server submit worker closed")

	for resp := range q.respCH {
		if resp == nil {
			return
		}
		resps := []*response{resp}

		for {
			shouldBreak := false
			select {
			case resp = <-q.respCH:
				if resp == nil {
					return
				}
				resps = append(resps, resp)
				if len(resps) > 256 {
					shouldBreak = true
				}
			default:
				shouldBreak = true
			}
			if shouldBreak {
				break
			}
		}
		hLen := 0
		for _, resp := range resps {
			hLen += len(resp.encodedHead)
		}
		var bufs net.Buffers
		batch := &batchHdrDesc{
			Version:    1,
			Cookie:     0, // TODO: resp should have a invalid cookie
			HeadLength: uint32(hLen),
			ChkSum:     0,
		}
		bufs = append(bufs, batch.Encode())
		for _, resp := range resps {
			bufs = append(bufs, resp.encodedHead)
		}
		writeBody := true
		for _, resp := range resps {
			if buf, ok := resp.Body.(*bytes.Buffer); resp.Body != nil && ok {
				bufs = append(bufs, buf.Bytes())
				writeBody = false
			}
		}

		for len(bufs) > 0 {
			_, err := bufs.WriteTo(q.conn)
			if err != nil {
				// TODO: reconnect
				panic(err)
			}
		}
		if writeBody {
			for _, resp := range resps {
				if resp.Body != nil {
					_, err := io.Copy(q.conn, resp.Body)
					if err != nil {
						// TODO: reconnect
						panic(err)
					}
				}
			}
		}
		for _, resp := range resps {
			respPool.Put(resp)
		}
	}
}

func (q *IOQueueBackend) Close() {
	q.conn.Close()
}

type Server struct {
	sync.Mutex
	listener net.Listener
	ip       string
	port     int
	dataGen  common.DataGen
	backends []*IOQueueBackend
}

func NewServer(ip, iname string, dg common.DataGen) (common.BlockServer, error) {
	ip, err := utils.FindLocalIP(ip, iname)
	if err != nil {
		return nil, err
	}

	svr := &Server{
		ip:       ip,
		port:     8000,
		dataGen:  dg,
		backends: make([]*IOQueueBackend, 0),
	}

	return svr, nil
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
		q := NewIOQueueBackend(s.dataGen, conn)
		s.backends = append(s.backends, q)
	}
	return err
}

func (s *Server) Close() {
	for _, b := range s.backends {
		b.Close()
	}
}
