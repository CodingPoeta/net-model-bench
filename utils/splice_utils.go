package utils

import (
	"github.com/hanwen/go-fuse/v2/splice"
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

type IsBuffer interface {
	io.Closer
	Iovec() [][]byte
}

type IsPipe interface {
	io.ReadCloser
	ReadFd() (fd uintptr)
	WriteTo(fd uintptr, n int) (int, error)
}

type Pipe struct {
	pair *splice.Pair
}

func (p *Pipe) ReadFd() uintptr {
	return p.pair.ReadFd()
}

func (p *Pipe) WriteTo(fd uintptr, n int) (int, error) {
	if p.pair == nil {
		return 0, io.EOF
	}
	return p.pair.WriteTo(fd, n)
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

type buffer []byte

func (b *buffer) Iovec() [][]byte {
	return [][]byte{*b}
}

func (b *buffer) Close() error {
	return nil
}

func (b *buffer) Read(p []byte) (n int, err error) {
	if len(*b) == 0 {
		return 0, io.EOF
	}
	n = copy(p, *b)
	if n < len(*b) {
		*b = (*b)[n:]
	} else {
		*b = nil
	}
	return
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
	_, err = pair.LoadFromAt(r.File(), size, offset)
	if err != nil {
		return nil, errors.Wrap(err, "pair load file")
	}
	return &Pipe{pair: pair}, nil
}

func PipeConn(r IsConn, size int) (IsPipe, error) {
	pair, err := splice.Get()
	if err != nil {
		return nil, errors.Wrap(err, "get pipe pair")
	}
	err = pair.Grow(alignSize(size))
	if err != nil {
		return nil, errors.Wrap(err, "grow pipe pair")
	}

	rawConn, err := r.SyscallConn()
	if err != nil {
		return nil, errors.Wrap(err, "get raw conn")
	}

	loaded := 0
	var loadError error

	err = rawConn.Read(func(fd uintptr) (done bool) {
		var n int
		n, loadError = pair.LoadFrom(fd, size-loaded)
		if loadError != nil {
			return loadError != syscall.EAGAIN && loadError != syscall.EINTR
		}
		loaded += n
		return loaded == size
	})

	if err == nil {
		err = loadError
	}
	if err != nil {
		return nil, errors.Wrap(err, "pair load file")
	}
	return &Pipe{pair: pair}, nil
}

func PipeBuffer(r IsBuffer, size int) (IsPipe, error) {
	pair, err := splice.Get()
	if err != nil {
		return nil, errors.Wrap(err, "get pipe pair")
	}
	err = pair.Grow(alignSize(size))
	if err != nil {
		return nil, errors.Wrap(err, "grow pipe pair")
	}

	for _, slice := range r.Iovec() {
		// TODO: use vmsplice to load buffer
		// There is a bug in vmsplice, it will cause data corruption
		// _, err = pair.LoadBuffer(r.Iovec(), size, splice.SPLICE_F_GIFT)
		_, err = syscall.Write(int(pair.WriteFd()), slice)
		if err != nil {
			return nil, errors.Wrap(err, "pair load buffer")
		}
	}

	return &Pipe{pair: pair}, nil
}

func alignSize(size int) int {
	pageSize := os.Getpagesize()
	return (size-1)/pageSize*pageSize + pageSize
}

func SpliceSendFile(conn net.Conn, file *os.File, size int) error {
	syscallConn, ok := conn.(syscall.Conn)
	if !ok {
		return errors.New("conn is not a syscall.Conn")
	}
	reader := &File{F: file}

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
		n, writeError = pipe.WriteTo(fd, int(uint64(size)-written))
		if writeError != nil {
			return writeError != syscall.EAGAIN && writeError != syscall.EINTR
		}
		written += uint64(n)
		return written == uint64(size)
	})
	if err == nil {
		err = writeError
	}
	return err
}
