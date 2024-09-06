package tcppool

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/codingpoeta/go-demo/common"
	"github.com/codingpoeta/go-demo/utils"
	"hash/crc32"
	"io"
	"math/rand"
	"net"
	"sync"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

type response struct {
	common.Response
	Header [13]byte
	Err    error
	tsz    int
	buf0   [5 << 20]byte
	buf1   [4 << 20]byte
}

func (r *response) Write(w io.Writer, comp, crc bool) error {
	if r.Err != nil {
		msg := r.Err.Error()
		buf := r.buf0[:12+len(msg)]
		binary.BigEndian.PutUint32(buf[:4], uint32(len(msg)))
		binary.BigEndian.PutUint32(buf[4:8], 0)
		binary.BigEndian.PutUint32(buf[8:12], 0)
		copy(buf[12:], []byte(msg))
		_, err := w.Write(buf[:12+len(msg)])
		return err
	}
	if comp {
		sz := utils.LZ4_compressBound(len(r.Body) + 1)
		copy(r.buf1[:len(r.Body)], r.Body)
		r.buf1[0] = utils.Letters[rand.Intn(len(utils.Letters))]
		buf := r.buf0[:12+sz]
		compsize := utils.LZ4_compress_default(r.buf1[:len(r.Body)], buf[12:])
		binary.BigEndian.PutUint32(buf[:4], compsize)
		binary.BigEndian.PutUint32(buf[4:8], uint32(len(r.Body)))
		binary.BigEndian.PutUint32(buf[8:12], crc32.Checksum(r.buf1[:len(r.Body)], crcTable))
		// fmt.Printf("compressed size: %d -> %d\n", len(r.Body), sz)
		// fmt.Println("Header:", buf[:12])
		_, err := w.Write(buf[:12+compsize])
		// _, err := w.Write([]byte("hello"))
		return err
	} else {
		buf := r.buf0[:12+len(r.Body)]
		binary.BigEndian.PutUint32(buf[:4], 0)
		binary.BigEndian.PutUint32(buf[4:8], uint32(len(r.Body)))
		if crc {
			binary.BigEndian.PutUint32(buf[8:12], crc32.Checksum(r.Body, crcTable))
		} else {
			binary.BigEndian.PutUint32(buf[8:12], 0)
		}
		copy(buf[12:], r.Body)
		_, err := w.Write(buf)
		return err
	}
}

func (r *response) Read(conn net.Conn) error {
	if _, err := io.ReadFull(conn, r.Header[:12]); err != nil {
		return err
	}
	compsize := binary.BigEndian.Uint32(r.Header[:4])
	osize := binary.BigEndian.Uint32(r.Header[4:8])
	if compsize > 20<<20 {
		return fmt.Errorf("payload is too big: %d", compsize)
	}

	r.tsz = int(compsize)
	if r.tsz == 0 {
		r.tsz = int(osize)
	}
	// fmt.Println("size:", size)
	payload := make([]byte, r.tsz)
	//deadline := time.Now().Add(1 * time.Second)
	//_ = conn.SetReadDeadline(deadline)
	var got, cnt int
	for got < len(payload) {
		// extend deadline for slow read
		//if time.Since(deadline) > -time.Millisecond*500 {
		//	deadline = time.Now().Add(1 * time.Second)
		//	_ = conn.SetReadDeadline(deadline)
		//}
		if n, err := conn.Read(payload[got:]); err != nil {
			return err
		} else {
			got += n
			cnt++
		}
	}
	if cnt > 0 {
		//fmt.Println("read count:", cnt)
	}
	if osize == 0 {
		r.Err = errors.New(string(payload))
		return nil
	}
	if compsize == 0 {
		r.Body = payload
	} else {
		r.Body = make([]byte, osize)
		n, err := utils.LZ4_decompress_fast(payload, r.Body)
		if err != nil {
			return err
		}
		if n != int(osize) {
			return fmt.Errorf("unexpected size: %d != %d", n, osize)
		}
	}
	r.CRCSum = binary.BigEndian.Uint32(r.Header[8:])
	if r.CRCSum != 0 {
		if s := crc32.Checksum(r.Body, crcTable); s != r.CRCSum {
			return fmt.Errorf("checksum %d != %d", s, r.CRCSum)
		}
	}
	// fmt.Println("decompressed size:", n)
	return nil
}

type Server struct {
	sync.Mutex
	listener net.Listener
	ip       string
	port     int
	dataGen  common.DataGen
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
	defer conn.Close()
	for {
		var req request
		if err := req.Read(conn); err != nil {
			fmt.Println(err)
			return
		}
		// fmt.Println("CMD:", req.CMD, "Key:", req.Key)
		var res response
		switch req.CMD {
		case 0:
			res.Body = s.dataGen.Get("key0")
		case 1:
			res.Body = s.dataGen.Get("key1")
		case 2:
			res.Body = s.dataGen.Get("key2")
		case 3:
			res.Body = s.dataGen.Get("key3")
		case 4:
			res.Body = s.dataGen.Get("key4")
		default:
			res.Err = errors.New("invalid command")
		}
		err := res.Write(conn, req.compressOn, req.crcOn)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}

func (s *Server) Close() {
}
