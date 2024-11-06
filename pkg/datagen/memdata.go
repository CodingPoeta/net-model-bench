package datagen

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/codingpoeta/net-model-bench/common"
	"github.com/codingpoeta/net-model-bench/utils"
	"golang.org/x/exp/rand"
)

type MemData struct {
	data map[string][]byte
}

func NewMemData() common.DataGen {
	buf := make([]byte, 1<<24)

	for i := 0; i < 1<<24; i++ {
		buf[i] = utils.Letters[rand.Intn(len(utils.Letters))]
	}
	for i := 0; i < 1<<17; i++ {
		for j := 0; j < 1<<4; j++ {
			io.WriteString(bytes.NewBuffer(buf[i<<7+j<<3:i<<7+j<<3]), "55AA5aa")
			buf[i<<7+j<<3+7] = byte(j)
		}
	}

	res := &MemData{
		data: map[string][]byte{
			"key0": buf[:4<<10],
			"key1": buf[4<<10 : 68<<10],
			"key2": buf[100<<10 : 228<<10],
			"key3": buf[11<<20 : 12<<20],
			"key4": buf[12<<20 : 16<<20],
		},
	}
	return res
}

func (m *MemData) Get(key string) []byte {
	res, ok := m.data[key]
	if ok {
		return res
	}
	s := strings.Split(key, "-")
	id, err := strconv.Atoi(s[1])
	if err != nil {
		panic(err)
	}

	var size int
	switch s[0] {
	case "key4":
		size = 1 << 22 // 4MiB
	case "key3":
		size = 1 << 20 // 1MiB
	case "key2":
		size = 1 << 17 // 128KiB
	case "key1":
		size = 1 << 16 // 64KiB
	default:
		size = 1 << 12 // 4KiB
	}
	buf := make([]byte, size)
	for i := 0; i < size>>7; i++ {
		for j := 0; j < 1<<4; j++ {
			io.WriteString(bytes.NewBuffer(buf[i<<7+j<<3:i<<7+j<<3]), "55AA5aa")
			buf[i<<7+j<<3+7] = byte(id)
		}
	}
	return buf
}

func (m *MemData) GetReadCloser(key string) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(m.Get(key)))
}

func (m *MemData) GetSize(key string) int {
	return len(m.data[key])
}
