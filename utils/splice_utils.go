package utils

import (
	"github.com/codingpoeta/net-model-bench/utils/splice"
	"github.com/pkg/errors"
	"io"
	"net"
	"os"
	"syscall"
)

type IsFile interface {
	io.Closer
	File() (fd uintptr)
}

type IsConn interface {
	io.Closer
	syscall.Conn
}

type IsPipe interface {
	io.ReadCloser
	ReadFd() (fd uintptr)
	WriteTo(fd uintptr, n int, flags int) (int, error)
}

type Pipe struct {
	pair *splice.Pair
}

func (p *Pipe) ReadFd() uintptr {
	return p.pair.ReadFd()
}

func (p *Pipe) WriteTo(fd uintptr, n int, flags int) (int, error) {
	if p.pair == nil {
		return 0, io.EOF
	}
	return p.pair.WriteTo(fd, n, flags)
}

func (p *Pipe) Read(b []byte) (int, error) {
	if p.pair == nil {
		return 0, io.EOF
	}
	return p.pair.Read(b)
}

func (p *Pipe) Close() error {
	if p.pair == nil {
		return nil
	}
	splice.Done(p.pair)
	return nil
}

type File struct {
	F *os.File
}

func (f *File) File() uintptr {
	return f.F.Fd()
}

func (f *File) Read(p []byte) (n int, err error) {
	return f.F.Read(p)
}

func (f *File) Close() error {
	return f.F.Close()
}

func PipeFile(r IsFile, offset int64, size int) (IsPipe, error) {
	pair, err := splice.Get()
	if err != nil {
		return nil, errors.Wrap(err, "get pipe pair")
	}
	err = pair.Grow(alignSize(size))
	if err != nil {
		return nil, errors.Wrap(err, "grow pipe pair")
	}
	_, err = pair.LoadFromAt(r.File(), size, &offset, splice.SPLICE_F_MOVE)
	if err != nil {
		return nil, errors.Wrap(err, "pair load file")
	}
	return &Pipe{pair: pair}, nil
}

func alignSize(size int) int {
	pageSize := os.Getpagesize()
	return size + pageSize - size%pageSize
}

func SpliceSendFile(conn net.Conn, file *os.File) error {
	syscallConn, ok := conn.(syscall.Conn)
	if !ok {
		return errors.New("conn is not a syscall.Conn")
	}
	reader := &File{F: file}
	size := uint64(4 << 20)

	pipe, err := PipeFile(reader, int64(0), int(size))
	if err != nil {
		// fail to load reader, fallback to normal copy
		return errors.Wrap(err, "pipe file")
	}
	defer pipe.Close()

	written := uint64(0)
	var writeError error
	dstRawConn, err := syscallConn.SyscallConn()
	if err != nil {
		return errors.Wrap(err, "syscall conn")
	}
	err = dstRawConn.Write(func(fd uintptr) (done bool) {
		var n int
		n, writeError = pipe.WriteTo(fd, int(size-written), splice.SPLICE_F_NONBLOCK|splice.SPLICE_F_MOVE)
		if writeError != nil {
			return writeError != syscall.EAGAIN && writeError != syscall.EINTR
		}
		written += uint64(n)
		return written == size
	})
	if err == nil {
		err = writeError
	}
	return err
}
