package common

import "sync/atomic"

type BodyBuffer struct {
	Buf     []byte
	ref     atomic.Int32
	Release func()
}

func (bb *BodyBuffer) Inc() {
	bb.ref.Add(1)
}

func (bb *BodyBuffer) Dec() {
	ref := bb.ref.Add(-1)
	if ref == 0 {
		bb.Release()
	}
}

type Response struct {
	Size   uint32
	Body   []byte
	CRCSum uint32
	BB     *BodyBuffer
}
