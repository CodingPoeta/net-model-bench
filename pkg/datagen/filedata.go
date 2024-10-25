package datagen

import (
	"os"

	"github.com/valyala/bytebufferpool"
)

type FileData struct {
	filename string
	len      int64
	pos      int64
	fh       *os.File
}

func NewFileData(filename string) DataGen {
	res := &FileData{
		filename: filename,
	}
	fi, err := os.Stat(filename)
	if err == nil {
		res.len = fi.Size()
		res.fh, err = os.OpenFile(filename, os.O_RDONLY, 0644)
		if err != nil {
			panic(err)
		}
	}
	return res
}

func (m *FileData) Get(key string) []byte {
	var len int64
	switch key {
	case "key0":
		len = 1 << 10
	case "key1":
		len = 1 << 20
	case "key2":
		len = 2 << 20
	case "key3":
		len = 3 << 20
	case "key4":
		len = 4 << 20
	}
	if m.pos+len > m.len {
		m.pos = 0
		rt, err := m.fh.Seek(0, 0)
		if err != nil {
			panic(err)
		}
		if rt != 0 {
			panic("seek failed")
		}
	}

	buf := bytebufferpool.Get()
	n, err := m.fh.Read(buf.Bytes()[:len])
	m.pos += int64(n)
	if err != nil {
		panic(err)
	} else if int64(n) != len {
		panic("short read")
	}

	return buf.Bytes()
}

func (m *FileData) GetSize(key string) int {
	switch key {
	case "key0":
		return 1 << 10
	case "key1":
		return 1 << 20
	case "key2":
		return 2 << 20
	case "key3":
		return 3 << 20
	case "key4":
		return 4 << 20
	}
	return 0
}
