package gonet

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"sync"

	"github.com/codingpoeta/go-demo/common"
	"github.com/codingpoeta/go-demo/utils"
	"github.com/panjf2000/gnet"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

type request struct {
	common.Request
	compressOn bool
	crcOn      bool
	Buf        [256]byte
}

func (r *request) Write(w io.Writer) error {
	buf := r.Buf[:]
	buf[0] = r.CMD & 0xF
	buf[1] = byte(len(r.Key))
	buf[2] = 0
	if r.compressOn {
		buf[2] |= 0x01
	}
	if r.crcOn {
		buf[2] |= 0x02
	}
	if len(r.Key) > 255 {
		buf[0] += byte(len(r.Key)>>8) << 4
	}
	copy(buf[3:], r.Key)
	if _, err := w.Write(buf[:len(r.Key)+3]); err != nil {
		return err
	}
	return nil
}

func (r *request) decode(buf []byte) {
	r.CMD = buf[0] & 0x0F
	size := int(buf[1]) + int(buf[0]>>4)<<8
	if buf[2]&0x01 != 0 {
		r.compressOn = true
	}
	if buf[2]&0x02 != 0 {
		r.crcOn = true
	}
	r.Key = string(buf[3 : 3+size])
}

type response struct {
	common.Response
	Header [13]byte
	Err    error
	tsz    int
}

func (r *response) encode(buf []byte, comp, crc bool) {
	header := buf[:12]
	if r.Err != nil {
		msg := r.Err.Error()
		binary.BigEndian.PutUint32(header[:4], uint32(len(msg)))
		binary.BigEndian.PutUint32(header[4:8], 0)
		binary.BigEndian.PutUint32(header[8:12], 0)
		copy(buf[12:], string(msg))
		return
	}
	binary.BigEndian.PutUint32(header[:4], 0)
	binary.BigEndian.PutUint32(header[4:8], uint32(len(r.Body)))
	binary.BigEndian.PutUint32(header[8:12], 0)
	copy(buf[12:], r.Body)
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

type server struct {
	*gnet.EventServer
	ip      string
	dataGen common.DataGen
}

func NewServer(ip, iname string, dg common.DataGen) (common.BlockServer, error) {
	ip, err := utils.FindLocalIP(ip, iname)
	if err != nil {
		return nil, err
	}
	return &server{
		ip:      ip,
		dataGen: dg,
	}, nil
}

func (s *server) Addr() string {
	return fmt.Sprintf("%s:8000", s.ip)
}

//	func (hc *server) Decode(c gnet.Conn) (out []byte, err error) {
//		buf := c.Read()
//		c.ResetBuffer()
//		req := &request{}
//		req.decode(buf)
//	}
//
// func (s *server) Encode(c gnet.Conn, buf []byte) (out []byte, err error) {
// }

var pool *sync.Pool

func init() {
	pool = &sync.Pool{
		New: func() any {
			return make([]byte, 4096*1024+12)
		},
	}
}

func (s *server) AfterWrite(c gnet.Conn, b []byte) {
	pool.Put(b)
}

func (s *server) React(frame []byte, c gnet.Conn) (out []byte, action gnet.Action) {
	req := &request{}
	req.decode(frame)
	if len(frame) != len(req.Key)+3 {
		panic(fmt.Sprintf("frame len is %d, expect %d", len(frame), len(req.Key)+3))
	}
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
	buf := pool.Get().([]byte)
	res.encode(buf[:], false, false)
	return buf[:12+len(res.Body)], action
}

func (s *server) Serve() (err error) {
	addr := fmt.Sprintf("%s:8000", s.ip)
	fmt.Printf("start listen on gnet %s\n", addr)
	err = gnet.Serve(s, fmt.Sprintf("tcp://%s", addr), gnet.WithMulticore(true))
	return err
}

func (s *server) Close() {
}
