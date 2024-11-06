package iorpc

import (
	"fmt"
	"os"

	"github.com/codingpoeta/net-model-bench/common"
	"github.com/codingpoeta/net-model-bench/pkg/iorpc"
)

var (
	ServiceNoop       iorpc.Service
	ServiceReadData   iorpc.Service
	ServiceReadMemory iorpc.Service
)

func NewDispatcher() *iorpc.Dispatcher {
	return iorpc.NewDispatcher()
}

func NewDispatcherForClient() *iorpc.Dispatcher {
	d := iorpc.NewDispatcher()
	addServiceNoop(d)
	addServiceReadData(d, nil)
	addServiceReadMemory(d)
	return nil
}

func addServiceNoop(dispatcher *iorpc.Dispatcher) {
	ServiceNoop, _ = dispatcher.AddService("Noop", func(clientAddr string, request iorpc.Request) (response *iorpc.Response, err error) {
		return &iorpc.Response{}, nil
	})
}

func addServiceReadData(dispatcher *iorpc.Dispatcher, dg common.DataGen) {
	ServiceReadData, _ = dispatcher.AddService(
		"ReadData",
		func(clientAddr string, request iorpc.Request) (*iorpc.Response, error) {
			request.Body.Close()
			cmd := uint64(4)
			ID := uint64(0)
			size := uint64(dg.GetSize(fmt.Sprintf("key%d", cmd)))
			offset := uint64(0)
			if request.Headers != nil {
				if headers := request.Headers.(*ReadHeaders); headers != nil {
					// fmt.Println("request.Headers", request.Headers)
					ID = headers.ID
					offset = headers.Offset
					cmd = headers.CMD
					size = uint64(dg.GetSize(fmt.Sprintf("key%d", cmd)))
					if headers.Size > 0 {
						size = headers.Size
					}
				}
			}
			// fmt.Println("----------reqID:  ", ID)
			var data *File
			if cmd == 4 {
				data = &File{file: dg.GetReadCloser(fmt.Sprintf("key%d-%d", cmd, ID)).(*os.File)}
			} else {
				data = &File{file: dg.GetReadCloser(fmt.Sprintf("key%d", cmd)).(*os.File)}
			}
			return &iorpc.Response{
				Headers: &ReadHeaders{
					CMD:    cmd,
					Offset: offset,
					Size:   size,
					ID:     ID,
				},
				Body: iorpc.Body{
					Offset:   offset,
					Size:     size,
					Reader:   data,
					NotClose: true,
				},
			}, nil
		},
	)
}

func addServiceReadMemory(dispatcher *iorpc.Dispatcher) {
	ServiceReadMemory, _ = dispatcher.AddService(
		"ReadMemory",
		func(clientAddr string, request iorpc.Request) (*iorpc.Response, error) {
			request.Body.Close()
			return &iorpc.Response{
				Body: iorpc.Body{},
			}, nil
		},
	)
}

type File struct {
	file *os.File
}

func (f *File) Close() error {
	return f.file.Close()
}

func (f *File) File() uintptr {
	return f.file.Fd()
}

func (f *File) Read(p []byte) (n int, err error) {
	return f.file.Read(p)
}
