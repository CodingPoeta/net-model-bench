package common

type Response struct {
	Size   uint32
	Body   []byte
	CRCSum uint32
}
