package iorpc

import (
	"fmt"
)

func sysWrite(fd int, p []byte) (n int, err error) {
	return 0, fmt.Errorf("not implemented")
}
