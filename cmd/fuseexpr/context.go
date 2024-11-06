package main

import (
	"sync"
	"time"

	"github.com/hanwen/go-fuse/v2/fuse"
)

var blockInter time.Duration = time.Second
var forceInter time.Duration = time.Minute * 6

type fuseContext struct {
	start    time.Time
	header   *fuse.InHeader
	gids     []uint32
	canceled bool
	cancel   <-chan struct{}
}

var contextPool = sync.Pool{
	New: func() interface{} {
		return &fuseContext{}
	},
}

func newContext(cancel <-chan struct{}, header *fuse.InHeader) *fuseContext {
	ctx := contextPool.Get().(*fuseContext)
	ctx.start = time.Now()
	ctx.canceled = false
	ctx.cancel = cancel
	ctx.header = header
	ctx.gids = nil
	return ctx
}

func releaseContext(ctx *fuseContext) {
	contextPool.Put(ctx)
}

func (c *fuseContext) Uid() uint32 {
	return uint32(c.header.Uid)
}

func (c *fuseContext) Gid() uint32 {
	return uint32(c.header.Gid)
}

func (c *fuseContext) Pid() uint32 {
	return uint32(c.header.Pid)
}

func (c *fuseContext) Duration() time.Duration {
	return time.Now().Sub(c.start)
}

func (c *fuseContext) Cancel() {
	c.canceled = true
}

func (c *fuseContext) Canceled() bool {
	used := c.Duration()
	if blockInter > 0 && used < blockInter {
		return false
	}
	if c.canceled {
		return true
	}
	select {
	case <-c.cancel:
		return true
	default:
		if used > forceInter {
			// logger.Infof("interrupt FUSE request %d forcely after %s", c.header.Unique, used)
			c.canceled = true
			return true
		}
		return false
	}
}
