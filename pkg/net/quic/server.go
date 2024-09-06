package quic

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	_ "net/http/pprof"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

type binds []string

func (b binds) String() string {
	return strings.Join(b, ",")
}

func (b *binds) Set(v string) error {
	*b = strings.Split(v, ",")
	return nil
}

// Size is needed by the /demo/upload handler to determine the size of the uploaded file
type Size interface {
	Size() int64
}

// See https://en.wikipedia.org/wiki/Lehmer_random_number_generator
func generatePRData(l int) []byte {
	res := make([]byte, l)
	seed := uint64(1)
	for i := 0; i < l; i++ {
		seed = seed * 48271 % 2147483647
		res[i] = byte(seed)
	}
	return res
}

func setupHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		const maxSize = 1 << 30 // 1 GB
		num, err := strconv.ParseInt(strings.ReplaceAll(r.RequestURI, "/", ""), 10, 64)
		if err != nil || num <= 0 || num > maxSize {
			w.WriteHeader(400)
			return
		}
		w.Write(generatePRData(int(num)))
	})

	return mux
}

func NewServer(tcp bool) {
	bs := binds{}

	if len(bs) == 0 {
		bs = binds{"localhost:6121"}
	}

	handler := setupHandler()

	var wg sync.WaitGroup
	wg.Add(len(bs))
	var certFile, keyFile string

	//certFile, keyFile = testdata.GetCertificatePaths()
	for _, b := range bs {
		fmt.Println("listening on", b)
		bCap := b
		go func() {
			var err error
			if tcp {
				err = http3.ListenAndServeTLS(bCap, certFile, keyFile, handler)
			} else {
				server := http3.Server{
					Handler:    handler,
					Addr:       bCap,
					QUICConfig: &quic.Config{},
				}
				err = server.ListenAndServeTLS(certFile, keyFile)
			}
			if err != nil {
				fmt.Println(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
