package main

type Ino uint64

type Attr struct {
	Flags     uint8
	Typ       uint8
	Mode      uint16
	Uid       uint32
	Gid       uint32
	Atime     int64
	Mtime     int64
	Ctime     int64
	Atimensec uint32
	Mtimensec uint32
	Ctimensec uint32
	Nlink     uint32
	Length    uint64
	Rdev      uint32
	Full      bool
}
