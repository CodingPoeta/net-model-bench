package iorpc

import "syscall"

func sysWrite(fd int, p []byte) (n int, err error) {
	return syscall.Write(fd, p)
}
