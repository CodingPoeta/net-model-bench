package datagen

import (
	"math/rand"

	"github.com/codingpoeta/go-demo/utils"
)

type MemData struct {
	data map[string][]byte
}

func NewMemData() DataGen {
	buf := make([]byte, 1<<24)

	for i := 0; i < 1<<24; i++ {
		buf[i] = utils.Letters[rand.Intn(len(utils.Letters))]
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
	return m.data[key]
}

func (m *MemData) GetSize(key string) int {
	return len(m.data[key])
}
