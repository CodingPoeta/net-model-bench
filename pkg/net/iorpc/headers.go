package iorpc

import (
	"encoding/binary"
	"io"
)

type ReadHeaders struct {
	CMD, Offset, Size uint64
	encodeBuf         [24]byte
}

func (h *ReadHeaders) Encode(w io.Writer) (int, error) {
	binary.BigEndian.PutUint64(h.encodeBuf[0:8], h.CMD)
	binary.BigEndian.PutUint64(h.encodeBuf[8:16], h.Offset)
	binary.BigEndian.PutUint64(h.encodeBuf[16:24], h.Size)
	return w.Write(h.encodeBuf[:])
}

func (h *ReadHeaders) Decode(b []byte) error {
	if len(b) != 24 {
		panic("")
	}
	h.CMD = binary.BigEndian.Uint64(b[0:8])
	h.Offset = binary.BigEndian.Uint64(b[8:16])
	h.Size = binary.BigEndian.Uint64(b[16:24])
	return nil
}
