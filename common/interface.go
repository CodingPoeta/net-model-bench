package common

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
}
