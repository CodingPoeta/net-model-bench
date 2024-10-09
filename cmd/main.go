package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codingpoeta/go-demo/pkg/net/gorpc"
	"github.com/codingpoeta/go-demo/pkg/net/perf"
	"github.com/felixge/fgprof"

	"github.com/codingpoeta/go-demo/common"
	"github.com/codingpoeta/go-demo/pkg/datagen"
	"github.com/codingpoeta/go-demo/pkg/net/grpc"
	"github.com/codingpoeta/go-demo/pkg/net/tcppool"
	"github.com/codingpoeta/go-demo/pkg/net/tcpsendfile"

	_ "net/http/pprof"

	"github.com/urfave/cli/v2"
)

var debugPort = 6060

func cmdServer() *cli.Command {
	return &cli.Command{
		Name:     "server",
		Usage:    "server",
		Category: "category2",
		Action: func(c *cli.Context) (err error) {
			fmt.Println("start server...")
			var svr common.BlockServer
			switch c.String("mode") {
			case "grpc":
				svr, err = grpc.NewServer(c.String("ip"), c.String("network"), datagen.NewMemData())
			case "gorpc":
				svr, err = gorpc.NewServer(c.String("ip"), c.String("network"), datagen.NewMemData())
			case "tcpsendfile":
				svr, err = tcpsendfile.NewServer(c.String("ip"), c.String("network"), datagen.NewMemData())
			case "perf":
				svr, err = perf.NewServer(c.String("ip"), c.String("network"), datagen.NewMemData())
			case "iorpc":
			// svr, err = iorpc.NewServer(c.String("ip"), c.String("network"), datagen.NewMemData())
			default:
				svr, err = tcppool.NewServer(c.String("ip"), c.String("network"), datagen.NewMemData())
			}
			if err != nil {
				fmt.Println(err)
				return err
			}
			defer svr.Close()
			return svr.Serve()
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "ip",
				Usage: "ip",
			},
			&cli.StringFlag{
				Name:  "network",
				Usage: "network",
			},
			&cli.StringFlag{
				Name:  "mode",
				Usage: "mode",
			},
		},
	}
}

func cmdClient() *cli.Command {
	return &cli.Command{
		Name:     "client",
		Usage:    "client",
		Category: "category2",
		Action: func(c *cli.Context) error {
			fmt.Println("client")
			var cli common.BlockClient
			switch c.String("mode") {
			case "grpc":
				cli = grpc.NewClient(c.String("addr"), c.Int("threads-per-con"), int(c.Int("threads")/c.Int("threads-per-con")))
			case "gorpc":
				cli = gorpc.NewClient(c.String("addr"), c.Int("threads"))
			case "iorpc": // TODO
			case "tcpsendfile":
				cli = tcpsendfile.NewClient(c.String("addr"), c.Int("threads"))
			case "perf":
				cli = perf.NewClient(c.String("addr"), c.Int("threads"))
			default:
				cli = tcppool.NewClient(c.String("addr"), c.Int("threads"), c.Bool("compress"), c.Bool("crc"))
			}
			cmd := uint8(c.Int("cmd"))
			defer cli.Close()
			var wg sync.WaitGroup
			var cnt atomic.Uint64
			var sz atomic.Uint64
			var lat atomic.Uint64

			for i := 0; i < c.Int("threads"); i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						since := time.Now()
						res, err := cli.Get(common.Request{CMD: cmd, Key: fmt.Sprint("testkey")})
						if err != nil {
							fmt.Println(err)
							break
						}
						// fmt.Println("data crc32:", res.crcsum)
						// fmt.Println("data len:", res.tsz, "bodysize", len(res.body))
						// cnt.Add(uint64(tsz))
						cnt.Add(1)
						sz.Add(uint64(len(res.Body)))
						lat.Add(uint64(time.Since(since)))
					}
				}()
			}
			go func() {
				for {
					time.Sleep(time.Second)
					_cnt := cnt.Load()
					cnt.Store(0)
					_sz := sz.Load()
					sz.Store(0)
					if _cnt == 0 {
						fmt.Printf("speed: %d B/s\n", _sz)
						continue
					}
					_lat := lat.Load() / _cnt
					lat.Store(0)

					if _sz>>30 > 10 {
						fmt.Printf("speed: %d GB/s; ", _sz>>30)
					} else if _sz>>20 > 10 {
						fmt.Printf("speed: %d MB/s; ", _sz>>20)
					} else if _sz>>10 > 10 {
						fmt.Printf("speed: %d KB/s; ", _sz>>10)
					} else {
						fmt.Printf("speed: %d B/s; ", _sz)
					}

					if _lat > 1e7 {
						fmt.Printf("latency: %d ms\n", _lat/1e6)
					} else if _lat > 1e4 {
						fmt.Printf("latency: %d us\n", _lat/1e3)
					} else {
						fmt.Printf("latency: %d ns\n", _lat)
					}
				}
			}()
			wg.Wait()

			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Usage: "addr",
			},
			&cli.IntFlag{
				Name:        "threads",
				Usage:       "threads",
				Aliases:     []string{"P"},
				DefaultText: "1",
			},
			&cli.IntFlag{
				Name:        "threads-per-con",
				Usage:       "threads-per-con",
				Aliases:     []string{"tpc"},
				DefaultText: "1",
			},
			&cli.IntFlag{
				Name:  "cmd",
				Usage: "cmd",
			},
			&cli.StringFlag{
				Name:  "mode",
				Usage: "mode",
			},
			&cli.BoolFlag{
				Name:    "compress",
				Usage:   "compress",
				Aliases: []string{"C"},
			},
			&cli.BoolFlag{
				Name:  "crc",
				Usage: "crc",
			},
		},
	}
}

func main() {
	mux := http.NewServeMux()
	mux.Handle("/debug/fgprof", fgprof.Handler())
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	go func() {
		for debugPort < 6100 {
			server := http.Server{
				Addr:    fmt.Sprintf(":%d", debugPort),
				Handler: mux,
			}
			log.Printf("starting debug server on port %d", debugPort)
			err := server.ListenAndServe()
			if err != nil {
				log.Printf("Error in http2 ListenAndServe: %v", err)
				// if err := server.ListenAndServeTLS("../output/server.crt", "../output/server.key"); err != nil {
				if strings.Contains(err.Error(), "bind: address already in use") {
					debugPort++
					continue
				}
			} else {
				break
			}
		}
	}()

	app := &cli.App{
		Name:  "app",
		Usage: "app",
		Commands: []*cli.Command{
			cmdServer(),
			cmdClient(),
		},
	}
	app.Run(os.Args)
}
