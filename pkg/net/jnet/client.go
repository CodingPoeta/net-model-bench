package jnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/codingpoeta/go-demo/common"
)

type batchHdrDesc struct {
	Version    uint32
	Cookie     uint64
	HeadLength uint32
	ChkSum     uint32   // bachhdr + request headers only
	Buf        [64]byte // cpu cache line
}

func (d *batchHdrDesc) Encode() []byte {
	buf := d.Buf
	binary.BigEndian.PutUint32(buf[0:4], d.Version)
	binary.BigEndian.PutUint64(buf[4:12], d.Cookie)
	binary.BigEndian.PutUint32(buf[12:16], d.HeadLength)
	binary.BigEndian.PutUint32(buf[16:20], d.ChkSum)
	return buf[:]
}

func (d *batchHdrDesc) Decode(b []byte) error {
	d.Version = binary.BigEndian.Uint32(b[:4])
	d.Cookie = binary.BigEndian.Uint64(b[4:12])
	d.HeadLength = binary.BigEndian.Uint32(b[12:16])
	d.ChkSum = binary.BigEndian.Uint32(b[16:20])
	return nil
}

type request struct {
	common.Request
	compressOn bool
	crcOn      bool
	ContentLen uint32
	Buf        [8]byte
	Body       io.Reader
	callback   func(*common.Response, error)
	resp       *response
	wait       chan struct{}

	encodedHead net.Buffers
}

func (r *request) Encode() net.Buffers {
	var buffs net.Buffers
	buf := r.Buf
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
	binary.BigEndian.PutUint32(buf[3:7], r.ContentLen)
	buffs = append(buffs, buf[:7], []byte(r.Key))
	return buffs
}

func (r *request) Decode(b []byte) (int, error) {
	r.CMD = b[0] & 0x0F
	size := int(b[1]) + int(b[0]>>4)<<8
	if b[2]&0x01 != 0 {
		r.compressOn = true
	}
	if b[2]&0x02 != 0 {
		r.crcOn = true
	}
	r.ContentLen = binary.BigEndian.Uint32(b[3:7])
	r.Key = string(b[7 : 7+size])
	return 7 + size, nil
}

type response struct {
	BatchId    uint64
	Idx        uint32
	ErrorCode  uint32
	ErrorMsg   string
	ContentLen uint32
	Buf        [64]byte
	Body       io.Reader

	encodedHead []byte
}

func (r *response) Encode() []byte {
	buf := r.Buf
	binary.BigEndian.PutUint64(buf[0:8], r.BatchId)
	binary.BigEndian.PutUint32(buf[8:12], r.Idx)
	binary.BigEndian.PutUint32(buf[12:16], r.ErrorCode)
	if r.ErrorCode != 0 {
		binary.BigEndian.PutUint32(buf[16:20], uint32(len(r.ErrorMsg)))
		r.Body = bytes.NewBuffer([]byte(r.ErrorMsg))
	} else {
		binary.BigEndian.PutUint32(buf[16:20], r.ContentLen)
	}
	return buf[:20]
}

func (r *response) Decode(b []byte) (int, error) {
	r.BatchId = binary.BigEndian.Uint64(b[0:8])
	r.Idx = binary.BigEndian.Uint32(b[8:12])
	r.ErrorCode = binary.BigEndian.Uint32(b[12:16])
	r.ContentLen = binary.BigEndian.Uint32(b[16:20])
	return 20, nil
}

type encodedRequest struct {
	head net.Buffers
	r    *request
}

type inflightBatchEntry struct {
	reqs []*request
	left int
}

type IOQueue struct {
	nextCookie      uint64
	conn            net.Conn
	reqCH           chan *request
	mu              sync.RWMutex
	inflightBatches map[uint64]*inflightBatchEntry
}

func NewIOQueue(addr string) (*IOQueue, error) {
	q := &IOQueue{
		reqCH:           make(chan *request, 2048),
		inflightBatches: make(map[uint64]*inflightBatchEntry),
	}
	dialer := &net.Dialer{Timeout: time.Second + time.Millisecond*100, KeepAlive: time.Minute}
	c, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	q.conn = c
	go q.submitWorker()
	go q.recvWorker()
	return q, nil
}

func (q *IOQueue) submit(req *request) *response {
	req.encodedHead = req.Encode()
	q.reqCH <- req
	<-req.wait
	return req.resp
}

func (q *IOQueue) submitWorker() {
	fmt.Println("submit worker started")
	defer fmt.Println("submit worker closed")

	// no batch in this version
	for req := range q.reqCH {
		requests := make([]*request, 1)
		if req == nil {
			return
		}
		requests[0] = req
		for {
			shouldBreak := false
			select {
			case req = <-q.reqCH:
				if req == nil {
					return
				}
				requests = append(requests, req)
				if len(requests) > 256 {
					shouldBreak = true
				}
			default:
				shouldBreak = true
			}
			if shouldBreak {
				break
			}
		}
		var bufs net.Buffers
		hlen := 0
		for _, req := range requests {
			for _, s := range req.encodedHead {
				hlen += len(s)
			}
		}
		batch := &batchHdrDesc{
			Version:    1,
			Cookie:     q.nextCookie,
			HeadLength: uint32(hlen),
			ChkSum:     0,
		}
		q.nextCookie += 1
		q.mu.Lock()
		q.inflightBatches[batch.Cookie] = &inflightBatchEntry{
			reqs: requests,
			left: len(requests),
		}
		q.mu.Unlock()

		bufs = append(bufs, batch.Encode())
		for _, req := range requests {
			bufs = append(bufs, req.encodedHead...)
		}
		for len(bufs) > 0 {
			_, err := bufs.WriteTo(q.conn)
			if err != nil {
				// TODO: reconnect
				panic(err)
			}
		}
		for _, req := range requests {
			if req.Body != nil {
				_, err := io.Copy(q.conn, req.Body)
				if err != nil {
					// TODO: reconnect
					panic(err)
				}
			}
		}
	}
}

func (q *IOQueue) recvWorker() {
	fmt.Println("recv worker started")
	defer fmt.Println("recv worker closed")

	var desc batchHdrDesc
	var respHeaderBuffer [20 * 512]byte
	resps := make([]*response, 0)
	for {
		resps = resps[:0]
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
		_, err = io.ReadFull(q.conn, respHeaderBuffer[:desc.HeadLength])
		if err != nil {
			panic(err)
		}
		left := desc.HeadLength
		idx := 0
		bodyLen := 0
		for left > 0 {
			var resp response
			n, err := resp.Decode(respHeaderBuffer[idx:])
			if err != nil {
				panic(err)
			}
			left -= uint32(n)
			idx += n
			bodyLen += int(resp.ContentLen)
			resps = append(resps, &resp)
		}

		bodyBuf := make([]byte, bodyLen)
		_, err = io.ReadFull(q.conn, bodyBuf)
		if err != nil {
			// TODO:
			panic(err)
		}

		off := 0
		for _, resp := range resps {
			if resp.ContentLen > 0 {
				resp.Body = bytes.NewBuffer(bodyBuf[off : off+int(resp.ContentLen)])
				off = off + int(resp.ContentLen)
			}
			q.mu.RLock()
			ents, ok := q.inflightBatches[resp.BatchId]
			if !ok {
				fmt.Printf("batch %d is not found\n", resp.BatchId)
				q.mu.RUnlock()
				continue
			}
			q.mu.RUnlock()
			req := ents.reqs[resp.Idx]
			ents.left -= 1
			if ents.left == 0 {
				q.mu.Lock()
				delete(q.inflightBatches, resp.BatchId)
				q.mu.Unlock()
			}
			req.resp = resp
			close(req.wait)

		}
	}
}

func (q *IOQueue) Close() {
	q.conn.Close()
}

type Client struct {
	sync.Mutex
	q          *IOQueue
	addr       string
	compressOn bool
	crcOn      bool
}

func NewClient(addr string, cons int, compressOn, crcOn bool) (common.BlockClient, error) {
	q, err := NewIOQueue(addr)
	if err != nil {
		return nil, err
	}
	cli := &Client{
		addr:       addr,
		compressOn: compressOn,
		crcOn:      crcOn,
		q:          q,
	}
	return cli, nil
}

func (c *Client) Close() {
	c.q.Close()
}

func (c *Client) Get(req_ common.Request) (*common.Response, error) {
	req := reqPool.Get().(*request)
	req.Request = req_
	req.compressOn = c.compressOn
	req.crcOn = c.crcOn
	req.wait = make(chan struct{})
	resp := c.q.submit(req)
	var err error
	if resp.ErrorCode != 0 {
		err = fmt.Errorf(resp.ErrorMsg)
	}
	buf := resp.Body.(*bytes.Buffer)
	body := buf.Bytes()

	return &common.Response{
		Body: body,
		Size: uint32(len(body)),
	}, err
}
