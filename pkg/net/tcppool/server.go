package tcppool

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"os"
	"sync"

	"github.com/codingpoeta/net-model-bench/common"
	"github.com/codingpoeta/net-model-bench/utils"

	"github.com/valyala/bytebufferpool"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

var payloadBufPool = &sync.Pool{
	New: func() any {
		return &common.BodyBuffer{
			Buf: make([]byte, 5120*1024),
		}
	},
}

type response struct {
	common.Response
	Header [13]byte
	Err    error
	tsz    int
}

func (r *response) Write(w io.Writer, comp, crc bool) error {
	var header [12]byte
	var buf = bytebufferpool.Get()
	defer bytebufferpool.Put(buf)
	//var buf1 = bytebufferpool.Get()
	if r.Err != nil {
		msg := r.Err.Error()
		binary.BigEndian.PutUint32(header[:4], uint32(len(msg)))
		binary.BigEndian.PutUint32(header[4:8], 0)
		binary.BigEndian.PutUint32(header[8:12], 0)
		_, _ = buf.Write(header[:])
		_, _ = buf.WriteString(msg)
		_, err := w.Write(buf.Bytes())
		return err
	}
	if comp {
		sz := utils.LZ4_compressBound(len(r.Body) + 1)
		_, _ = buf.Write(make([]byte, 12+sz))
		//copy(buf1.Bytes()[:len(r.Body)], r.Body)
		//buf1.Bytes()[0] = utils.Letters[rand.Intn(len(utils.Letters))]
		compsize := utils.LZ4_compress_default(r.Body, buf.Bytes()[12:])
		binary.BigEndian.PutUint32(header[:4], compsize)
		binary.BigEndian.PutUint32(header[4:8], uint32(len(r.Body)))
		//binary.BigEndian.PutUint32(header[8:12], crc32.Checksum(buf1.Bytes()[:len(r.Body)], crcTable))
		binary.BigEndian.PutUint32(header[8:12], crc32.Checksum(r.Body, crcTable))
		// fmt.Printf("compressed size: %d -> %d\n", len(r.Body), sz)
		// fmt.Println("Header:", header[:12])
		copy(buf.Bytes()[:12], header[:])
		_, err := w.Write(buf.Bytes()[:12+compsize])
		// _, err := w.Write([]byte("hello"))
		return err
	} else {
		binary.BigEndian.PutUint32(header[:4], 0)
		binary.BigEndian.PutUint32(header[4:8], uint32(len(r.Body)))
		if crc {
			binary.BigEndian.PutUint32(header[8:12], crc32.Checksum(r.Body, crcTable))
		} else {
			binary.BigEndian.PutUint32(header[8:12], 0)
		}
		//_, _ = buf.Write(header[:])
		//_, _ = buf.Write(r.Body)
		//_, err := w.Write(buf.Bytes())
		var bufs net.Buffers
		bufs = append(bufs, header[:])
		bufs = append(bufs, r.Body)
		_, err := bufs.WriteTo(w)
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
	payloadBuf := payloadBufPool.Get().(*common.BodyBuffer)
	payloadBuf.Release = func() {
		payloadBufPool.Put(payloadBuf)
	}
	payload := payloadBuf.Buf[:r.tsz]
	//deadline := time.Now().Add(1 * time.Second)
	//_ = conn.SetReadDeadline(deadline)
	var got, cnt int
	for got < len(payload) {
		// extend deadline for slow read
		//if time.Since(deadline) > -time.Millisecond*500 {
		//	deadline = time.Now().Add(1 * time.Second)
		//	_ = conn.SetReadDeadline(deadline)
		//}
		if n, err := io.ReadFull(conn, payload[got:]); err != nil {
			// if n, err := conn.Read(payload[got:]); err != nil {
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
		r.BB = payloadBuf
		r.BB.Inc()
	} else {
		tmp := payloadBufPool.Get().(*common.BodyBuffer)
		tmp.Release = func() { payloadBufPool.Put(tmp) }
		r.Body = tmp.Buf[:osize]
		n, err := utils.LZ4_decompress_fast(payload, r.Body)
		r.BB = tmp
		r.BB.Inc()
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
	mode     common.ServerMode
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
	// bfsz := 10 << 20
	// conn.(*net.TCPConn).SetWriteBuffer(bfsz)
	// conn.(*net.TCPConn).SetReadBuffer(bfsz)
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
