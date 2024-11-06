package main

import (
	"runtime"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

func main() {
	if runtime.GOOS == "linux" {
		err := grantAccess()
		if err != nil {
			logger.Errorf("grant access to /dev/fuse: %s", err)
		}
		ensureFuseDev()
	}

	sFS := NewStackFS("/tmp/fuse", 1024)
	opts := sFS.GetFuseOpts()
	fssrv, err := fuse.NewServer(sFS, "./mnt", opts)
	if err != nil {
		panic(err)
	}
	fssrv.Serve()
}
