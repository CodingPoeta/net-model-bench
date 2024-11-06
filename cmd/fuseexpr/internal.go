package main

const (
	minInternalNode = 0x7FFFFFFFFFFFF0

	statInode = minInternalNode + iota
	oplogInode
	ophistoryInode
	configInode
	configInode2
	zonemapInode
	controlInode
	heatmapInode
	trashInode
)

type internalNode struct {
	inode Ino
	name  string
	attr  *Attr
	show  bool
}

var internalNodes = []*internalNode{
	{controlInode, ".control", &Attr{Mode: 0666}, false},
	{oplogInode, ".oplog", &Attr{Mode: 0400}, false},
	{ophistoryInode, ".ophistory", &Attr{Mode: 0400}, false},
	{configInode2, ".jfsconfig", &Attr{Mode: 0400}, false},
	{zonemapInode, ".zonemap", &Attr{Mode: 0444}, false},
	{heatmapInode, ".heatmap", &Attr{Mode: 0444}, false},

	{statInode, ".stats", &Attr{Mode: 0444}, true},
	{oplogInode, ".accesslog", &Attr{Mode: 0400}, true},
	{configInode, ".config", &Attr{Mode: 0400}, true},
}

func IsSpecialNode(ino Ino) bool {
	return ino >= minInternalNode
}
