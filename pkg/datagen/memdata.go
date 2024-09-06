package datagen

import (
	"math/rand"

	"github.com/codingpoeta/go-demo/utils"
)

type MemData struct {
	data map[string][]byte
}

func NewMemData() *MemData {
	buf := make([]byte, 1<<24)

	for i := 0; i < 1<<24; i++ {
		buf[i] = utils.Letters[rand.Intn(len(utils.Letters))]
	}

	res := &MemData{
		data: map[string][]byte{
			"key0": buf[:1<<10],
			"key1": buf[1<<20 : 2<<20],
			"key2": buf[2<<20 : 4<<20],
			"key3": buf[4<<20 : 7<<20],
			"key4": buf[7<<20 : 11<<20],
		},
	}
	return res
}

func (m *MemData) Get(key string) []byte {
	return m.data[key]
}
