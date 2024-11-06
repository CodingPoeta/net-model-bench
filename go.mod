module github.com/codingpoeta/net-model-bench

require (
	github.com/hanwen/go-fuse/v2 v2.1.1-0.20210611132105-24a1dfe6b4f8
	github.com/juicedata/juicefs v1.1.2
	github.com/panjf2000/gnet v1.6.7
	github.com/pkg/errors v0.9.1
	github.com/quic-go/quic-go v0.45.1
	github.com/sirupsen/logrus v1.9.0
	github.com/stretchr/testify v1.8.1
	github.com/urfave/cli/v2 v2.19.3
	github.com/valyala/bytebufferpool v1.0.0
	github.com/valyala/gorpc v0.0.0-20160519171614-908281bef774
	golang.org/x/sync v0.7.0
	google.golang.org/grpc v1.64.0
	google.golang.org/protobuf v1.34.1
)

require (
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/DataDog/zstd v1.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/pprof v0.0.0-20240227163752-401108e1b7e7 // indirect
	github.com/hungys/go-lz4 v0.0.0-20170805124057-19ff7f07f099 // indirect
	github.com/onsi/ginkgo/v2 v2.9.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/mock v0.4.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.20.0 // indirect
	golang.org/x/crypto v0.23.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	golang.org/x/tools v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240610135401-a8a62080eff3 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/urfave/cli/v2 v2.25.3 => github.com/juicedata/cli/v2 v2.25.4-0.20230526070816-8aff66437fa8

replace github.com/hexilee/iorpc v0.0.0-20221111023153-6594c32b0c69 => github.com/winglq/iorpc v0.0.0-20241031025143-df0cab627377

replace github.com/hanwen/go-fuse/v2 v2.1.1-0.20210611132105-24a1dfe6b4f8 => github.com/juicedata/go-fuse/v2 v2.1.1-0.20241105033405-a7fea3786d15

replace github.com/codingpoeta/net-model-bench => ./

go 1.21.0
