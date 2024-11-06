package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/hanwen/go-fuse/v2/fuse"
)

type stackFS struct {
	fuse.RawFileSystem

	basePath  string
	capacity  int
	available int

	baseDirAttr fuse.Attr
}

func NewStackFS(basePath string, capacity int) *stackFS {
	fs := &stackFS{
		RawFileSystem: fuse.NewDefaultRawFileSystem(),
		basePath:      basePath,
		capacity:      capacity,
		available:     capacity,
	}
	// check if basePath exists
	var f os.FileInfo
	var err error
	if f, err = os.Stat(basePath); os.IsNotExist(err) {
		err = os.MkdirAll(basePath, 0755)
		if err != nil {
			panic(err)
		}
	}

	fs.baseDirAttr.Ino = 1
	fs.baseDirAttr.Size = uint64(16 << 30)
	fs.baseDirAttr.Blocks = uint64(16 << 30 >> 9)
	fs.baseDirAttr.Atime = uint64(f.ModTime().Unix())
	fs.baseDirAttr.Mtime = uint64(f.ModTime().Unix())
	fs.baseDirAttr.Ctime = uint64(f.ModTime().Unix())
	// fs.baseDirAttr.Atimensec = uint32(f.ModTime().Nanosecond())
	// fs.baseDirAttr.Mtimensec = uint32(f.ModTime().Nanosecond())
	// fs.baseDirAttr.Ctimensec = uint32(f.ModTime().Nanosecond())
	// fs.baseDirAttr.Mode = uint32(f.Mode())
	fs.baseDirAttr.Mode = uint32(0x41ff)
	// fs.baseDirAttr.Nlink = uint32(f.Sys().(*syscall.Stat_t).Nlink)
	fs.baseDirAttr.Nlink = 0x12
	// fs.baseDirAttr.Uid = uint32(f.Sys().(*syscall.Stat_t).Uid)
	// fs.baseDirAttr.Gid = uint32(f.Sys().(*syscall.Stat_t).Gid)
	fs.baseDirAttr.Uid = 0
	fs.baseDirAttr.Gid = 0
	// fs.baseDirAttr.Blksize = uint32(f.Sys().(*syscall.Stat_t).Blksize)
	fs.baseDirAttr.Blksize = 1 << 16

	return fs
}

func (fs *stackFS) GetFuseOpts() *fuse.MountOptions {
	return &fuse.MountOptions{
		Name:        "stackfs",
		FsName:      "stackfs: " + fs.basePath,
		DirectMount: true,
		AllowOther:  true,
		Debug:       true,

		Options: []string{"default_permissions"},
	}
}

func (fs *stackFS) debug(caller string) {
	fmt.Println("stackFS: ", caller)
	return
}

func (fs *stackFS) Lookup(cancel <-chan struct{}, header *fuse.InHeader, name string, out *fuse.EntryOut) (status fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) GetAttr(cancel <-chan struct{}, in *fuse.GetAttrIn, out *fuse.AttrOut) (code fuse.Status) {
	fs.debug(printMyName())
	if in.NodeId == 1 {
		out.AttrValid = 1
		out.Attr = fs.baseDirAttr
		return fuse.OK
	}

	return fuse.ENOENT
}

func (fs *stackFS) SetAttr(cancel <-chan struct{}, in *fuse.SetAttrIn, out *fuse.AttrOut) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Mknod(cancel <-chan struct{}, in *fuse.MknodIn, name string, out *fuse.EntryOut) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Mkdir(cancel <-chan struct{}, in *fuse.MkdirIn, name string, out *fuse.EntryOut) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Unlink(cancel <-chan struct{}, header *fuse.InHeader, name string) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Rmdir(cancel <-chan struct{}, header *fuse.InHeader, name string) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Rename(cancel <-chan struct{}, in *fuse.RenameIn, oldName string, newName string) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Link(cancel <-chan struct{}, in *fuse.LinkIn, name string, out *fuse.EntryOut) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Symlink(cancel <-chan struct{}, header *fuse.InHeader, target string, name string, out *fuse.EntryOut) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Readlink(cancel <-chan struct{}, header *fuse.InHeader) (out []byte, code fuse.Status) {
	fs.debug(printMyName())
	return nil, fuse.ENOENT
}

func (fs *stackFS) Access(cancel <-chan struct{}, in *fuse.AccessIn) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) GetXAttr(cancel <-chan struct{}, header *fuse.InHeader, attr string, dest []byte) (sz uint32, code fuse.Status) {
	fs.debug(printMyName())
	return 0, fuse.ENOENT
}

func (fs *stackFS) ListXAttr(cancel <-chan struct{}, header *fuse.InHeader, dest []byte) (uint32, fuse.Status) {
	fs.debug(printMyName())
	return 0, fuse.ENOENT
}

func (fs *stackFS) SetXAttr(cancel <-chan struct{}, in *fuse.SetXAttrIn, attr string, data []byte) fuse.Status {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) RemoveXAttr(cancel <-chan struct{}, header *fuse.InHeader, attr string) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Create(cancel <-chan struct{}, in *fuse.CreateIn, name string, out *fuse.CreateOut) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Open(cancel <-chan struct{}, in *fuse.OpenIn, out *fuse.OpenOut) (status fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Read(cancel <-chan struct{}, in *fuse.ReadIn, buf []byte) (fuse.ReadResult, fuse.Status) {
	fs.debug(printMyName())
	return nil, fuse.ENOENT
}

func (fs *stackFS) Lseek(cancel <-chan struct{}, in *fuse.LseekIn, out *fuse.LseekOut) fuse.Status {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) GetLk(cancel <-chan struct{}, in *fuse.LkIn, out *fuse.LkOut) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) SetLk(cancel <-chan struct{}, in *fuse.LkIn) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) SetLkw(cancel <-chan struct{}, in *fuse.LkIn) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Flock(cancel <-chan struct{}, in *fuse.LkIn, block bool) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Release(cancel <-chan struct{}, in *fuse.ReleaseIn) {

}

func (fs *stackFS) Write(cancel <-chan struct{}, in *fuse.WriteIn, data []byte) (written uint32, code fuse.Status) {
	fs.debug(printMyName())
	return 0, fuse.ENOENT
}

func (fs *stackFS) CopyFileRange(cancel <-chan struct{}, in *fuse.CopyFileRangeIn) (written uint32, code fuse.Status) {
	fs.debug(printMyName())
	return 0, fuse.ENOENT
}

func (fs *stackFS) Flush(cancel <-chan struct{}, in *fuse.FlushIn) fuse.Status {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Fsync(cancel <-chan struct{}, in *fuse.FsyncIn) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) Fallocate(cancel <-chan struct{}, in *fuse.FallocateIn) (code fuse.Status) {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) openDir(path string, out *fuse.OpenOut) (fh uint64, status fuse.Status) {
	os.Open(path)
	return 0, fuse.ENOENT
}

func (fs *stackFS) OpenDir(cancel <-chan struct{}, in *fuse.OpenIn, out *fuse.OpenOut) (status fuse.Status) {
	fs.debug(printMyName())
	if in.NodeId == 1 {
		// path := fs.basePath
		// fh, err :=
		return fuse.OK
	}
	return fuse.ENOENT
}

func (fs *stackFS) ReadDir(cancel <-chan struct{}, in *fuse.ReadIn, out *fuse.DirEntryList) fuse.Status {
	// inode := in.NodeId
	// fh := in.Fh
	// size := in.Size
	// offset := in.Offset

	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) ReadDirPlus(cancel <-chan struct{}, in *fuse.ReadIn, out *fuse.DirEntryList) fuse.Status {
	fs.debug(printMyName())
	return fuse.ENOENT
}

func (fs *stackFS) ReleaseDir(in *fuse.ReleaseIn) {
}

func (fs *stackFS) StatFs(cancel <-chan struct{}, in *fuse.InHeader, out *fuse.StatfsOut) (code fuse.Status) {
	fs.debug(printMyName())

	return fuse.ENOENT
}

func printMyName() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}
