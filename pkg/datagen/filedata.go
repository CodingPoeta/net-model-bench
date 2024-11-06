package datagen

import (
	"fmt"
	"io"
	"os"

	"github.com/codingpoeta/net-model-bench/common"

	"github.com/valyala/bytebufferpool"
)

type FileData struct {
	basePath string
	len      int64
	pos      int64
}

func NewFileData(basePath string) common.DataGen {
	res := &FileData{
		basePath: basePath,
	}
	for i := 0; i < 5; i++ {
		file, err := os.Create(fmt.Sprintf("%s/key%d", basePath, i))
		if err != nil {
			panic(err)
		}
		file.Write(NewMemData().Get(fmt.Sprintf("key%d", i)))
		file.Close()
	}
	for k := 0; k < 5; k++ {
		for id := 1; id <= 50; id++ {
			if _, err := os.Stat(fmt.Sprintf("%s/key%d-%d", basePath, k, id)); os.IsNotExist(err) {
			} else if err != nil {
				panic(err)
			} else {
				continue
			}
			file, err := os.Create(fmt.Sprintf("%s/key%d-%d", basePath, k, id))
			if err != nil {
				panic(err)
			}
			file.Write(NewMemData().Get(fmt.Sprintf("key%d-%d", k, id)))
			file.Close()
		}
	}
	return res
}

func (m *FileData) Get(key string) []byte {
	reader := m.GetReadCloser(key)
	defer reader.Close()

	buf := bytebufferpool.Get()
	n, err := reader.Read(buf.Bytes()[:m.GetSize(key)])
	if err != nil {
		panic(err)
	} else if n != m.GetSize(key) {
		panic("short read")
	}

	return buf.Bytes()
}

func (m *FileData) GetReadCloser(key string) io.ReadCloser {
	fh, err := os.OpenFile(m.basePath+key, os.O_RDONLY, 0644)
	if err != nil {
		panic(err)
	}
	return fh
}

func (m *FileData) GetSize(key string) int {
	switch key {
	case "key0":
		return 4 << 10
	case "key1":
		return 64 << 10
	case "key2":
		return 128 << 10
	case "key3":
		return 1 << 20
	case "key4":
		return 4 << 20
	}
	return 0
}
