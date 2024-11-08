package iorpc

import (
	"io"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/v2/splice"

	"github.com/pkg/errors"
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
	WriteTo(fd uintptr, n int) (int, error)
}

type IsBuffer interface {
	io.Closer
	Iovec() [][]byte
}

type noopBody struct{}

func (noopBody) Read(p []byte) (int, error) {
	return 0, io.EOF
}

func (noopBody) Close() error {
	return nil
}

func PipeFile(r IsFile, offset int64, size int) (IsPipe, error) {
	pair, err := splice.Get()
	if err != nil {
		return nil, errors.Wrap(err, "get pipe pair")
	}
	err = pair.Grow(alignSize(size) * 2)
	if err != nil {
		return nil, errors.Wrap(err, "grow pipe pair")
	}
	_, err = pair.LoadFromAt(r.File(), size, offset)
	if err != nil {
		return nil, errors.Wrap(err, "pair load file")
	}
	return &Pipe{pair: pair}, nil
}

func alignSize(size int) int {
	// added extra page if it's already aligned
	pageSize := os.Getpagesize()
	return size + pageSize - size%pageSize
}

func PipeConn(r IsConn, size int) (IsPipe, error) {
	rawConn, err := r.SyscallConn()
	if err != nil {
		return nil, errors.Wrap(err, "get raw conn")
	}

	pair, err := splice.Get()
	if err != nil {
		return nil, errors.Wrap(err, "get pipe pair")
	}
	err = pair.Grow(alignSize(size) * 2)
	if err != nil {
		splice.Drop(pair)
		return nil, errors.Wrap(err, "grow pipe pair")
	}

	var loaded int
	var eno error

	err = rawConn.Read(func(fd uintptr) (done bool) {
		var n int
		for {
			var syserr *os.SyscallError
			n, eno = pair.LoadFrom(fd, size-loaded)
			syserr, _ = eno.(*os.SyscallError)
			if syserr != nil {
				if syserr.Err == syscall.EINTR {
					continue
				}
				return syserr.Err != syscall.EAGAIN
			}
			loaded += n
			if loaded < size {
				continue
			}
			return true
		}
	})

	if err == nil {
		err = eno
	}
	if err != nil {
		splice.Done(pair)
		return nil, errors.Wrap(err, "pair load file")
	}
	return &Pipe{pair: pair}, nil
}

func PipeBuffer(r IsBuffer, size int) (IsPipe, error) {
	pair, err := splice.Get()
	if err != nil {
		return nil, errors.Wrap(err, "get pipe pair")
	}
	err = pair.Grow(alignSize(size) * 2)
	if err != nil {
		splice.Drop(pair)
		return nil, errors.Wrap(err, "grow pipe pair")
	}

	// TODO: use writev or vmsplice
	// _, err = pair.LoadBuffer(r.Iovec(), size, 0)
	for _, slice := range r.Iovec() {
		_, err = sysWrite(int(pair.WriteFd()), slice)
		if err != nil {
			splice.Done(pair)
			return nil, errors.Wrap(err, "pair load buffer")
		}
	}

	return &Pipe{pair: pair}, nil
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

type Body struct {
	Offset, Size uint64
	Reader       io.ReadCloser
	NotClose     bool
}

func (b *Body) Reset() {
	b.Offset, b.Size, b.Reader, b.NotClose = 0, 0, nil, false
}

func (b *Body) Close() error {
	if b.NotClose || b.Reader == nil {
		return nil
	}
	return b.Reader.Close()
}

func (b *Body) spliceTo(w io.Writer) (bool, error) {
	syscallConn, ok := w.(syscall.Conn)
	if !ok {
		return false, nil
	}

	dstRawConn, err := syscallConn.SyscallConn()
	if err != nil {
		return false, nil
	}

	var pipe IsPipe
	switch reader := b.Reader.(type) {
	case IsPipe:
		pipe = reader
	case IsFile:
		pipe, err = PipeFile(reader, int64(b.Offset), int(b.Size))
	case IsConn:
		pipe, err = PipeConn(reader, int(b.Size))
	case IsBuffer:
		pipe, err = PipeBuffer(reader, int(b.Size))
	default:
		return false, nil
	}

	if err != nil {
		// fail to load reader, fallback to normal copy
		// FIXME: reader could be read
		return false, nil
	}
	defer pipe.Close()

	var written int
	var eno error
	err = dstRawConn.Write(func(fd uintptr) (done bool) {
		var n int
		for {
			var syserr *os.SyscallError
			n, eno = pipe.WriteTo(fd, int(b.Size)-written)
			syserr, _ = eno.(*os.SyscallError)
			if syserr != nil {
				if syserr.Err == syscall.EINTR {
					continue
				}
				return syserr.Err != syscall.EAGAIN
			}
			written += n
			if written < int(b.Size) {
				continue
			}
			return true
		}
	})
	if err == nil {
		err = eno
	}
	return true, err
}
