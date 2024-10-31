package common

import "io"

type BlockServer interface {
	Serve() error
	Addr() string
	Close()
}

type BlockClient interface {
	Get(req Request) (*Response, error)
	Close()
}

type DataGen interface {
	Get(key string) []byte
	GetReader(key string) io.Reader
	GetSize(key string) int
}

type ServerMode int

const (
	MODE_SENDBUF = iota
	MODE_SENDFILE
	MODE_SPLICE
)
