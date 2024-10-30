package quic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codingpoeta/net-model-bench/common"
	"github.com/quic-go/quic-go"
)

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
	// fmt.Printf("CMD: %d, Key: %s\n", r.CMD, r.Key)
	if _, err := w.Write(buf[:len(r.Key)+3]); err != nil {
		return err
	}
	return nil
}

func (r *request) Read(str io.Reader) error {
	if _, err := io.ReadFull(str, r.Buf[:3]); err != nil {
		return err
	}
	r.CMD = r.Buf[0] & 0x0F
	size := int(r.Buf[1]) + int(r.Buf[0]>>4)<<8
	if r.Buf[2]&0x01 != 0 {
		r.compressOn = true
	}
	if r.Buf[2]&0x02 != 0 {
		r.crcOn = true
	}

	buf := r.Buf[:size]
	if _, err := io.ReadFull(str, buf); err != nil {
		return err
	}
	r.Key = string(buf)
	// fmt.Println("CMD:", r.CMD, "Key:", r.Key)
	return nil
}

type quicConn struct {
	conn quic.EarlyConnection
	str  quic.Stream
}

type Client struct {
	sync.Mutex
	addr       *net.UDPAddr
	compressOn bool
	crcOn      bool
	quicConf   *quic.Config
	quicConns  chan *quicConn
}

func NewClient(addr string, cons int, compressOn, crcOn bool) common.BlockClient {
	port, err := strconv.Atoi(strings.Split(addr, ":")[1])
	if err != nil {
		return nil
	}
	return &Client{
		addr:       &net.UDPAddr{IP: net.ParseIP(strings.Split(addr, ":")[0]), Port: port},
		compressOn: compressOn,
		crcOn:      crcOn,
		quicConns:  make(chan *quicConn, cons),
		quicConf:   &quic.Config{Allow0RTT: true},
	}
}

func NewTLSConfig() (*tls.Config, error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"camera-quic-server"},
	}, nil
}

func GetTLSConfig() (*tls.Config, error) {
	return NewTLSConfig()

	cert, err := tls.LoadX509KeyPair("./server.crt", "./server.key")
	if err != nil {
		return nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

func (c *Client) getConn() (*quicConn, error) {
	var conn *quicConn
	select {
	case conn = <-c.quicConns:
	default:
		conn = &quicConn{}
		// one connect at a time
		c.Lock()
		defer c.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		tlsConf, err := GetTLSConfig()
		if err != nil {
			return nil, err
		}
		tlsConf.InsecureSkipVerify = true
		tlsConf.ClientSessionCache = tls.NewLRUClientSessionCache(100)

		// 1. Use this tls.Config to establish the first connection to the server
		// and receive a session ticket ...
		// 2. Dial another connection to the same server
		tmpconn, err := quic.DialAddrEarly(ctx, c.addr.String(), tlsConf, c.quicConf)
		conn.conn = tmpconn
		if err != nil {
			return nil, err
		}
		// ... error handling
		// Check if 0-RTT is being used
		uses0RTT := conn.conn.ConnectionState().Used0RTT
		fmt.Println("0-RTT used:", uses0RTT)
		// If 0-RTT was used, DialEarly returned immediately.
		// Open a stream and send some application data in 0-RTT ...
		conn.str, err = conn.conn.OpenStream()
		if err != nil {
			return nil, err
		}
		fmt.Println("stream opened")
	}
	return conn, nil
}

func (c *Client) withConn(f func(conn *quicConn) error) error {
	conn, err := c.getConn()
	if err != nil {
		return err
	}
	err = f(conn)
	if err != nil {
		return err
	}
	select {
	case c.quicConns <- conn:
	default:
		_ = conn.conn.CloseWithError(0x42, "I don't want to talk to you anymore 🙉")
	}
	return nil
}

func (c *Client) Close() {
	close(c.quicConns)
}

func (c *Client) Get(req_ common.Request) (*common.Response, error) {
	var res response

	err := c.withConn(func(conn *quicConn) error {
		req := request{Request: req_, compressOn: c.compressOn, crcOn: c.crcOn}
		// fmt.Println("CMD:", req.CMD, "Key:", req.Key)
		if err := req.Write(conn.str); err != nil {
			fmt.Println("write error:", err)
			return err
		}
		if err := res.Read(conn.str); err != nil {
			fmt.Println("read error:", err)
			return err
		} else if res.Err != nil {
			fmt.Println("response error:", res.Err)
			return res.Err
		}
		return nil
	})
	return &(res.Response), err
}
