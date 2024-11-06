package main

import (
	"os"
	"sync"
)

type fileHandle struct {
	ino Ino
	f   *os.File
}

var (
	fileHandles map[Ino][]*fileHandle
	handleLock  sync.Mutex
	nextfh      uint64 = 1
)
