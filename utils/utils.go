package utils

import (
	"errors"
	"fmt"
	"github.com/juicedata/juicefs/pkg/compress"
	"net"
	"strings"
)

func FindLocalIP(mask string, iname string) (string, error) {
	for strings.HasSuffix(mask, ".0") {
		mask = mask[:len(mask)-2]
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 && iname == "" && mask == "" {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		if iname != "" && iface.Name != iname && !strings.HasPrefix(iface.Name, iname+".") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			if !strings.HasPrefix(ip.String(), mask) {
				continue
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}

type LZ4 = compress.LZ4

var lz4 = LZ4{}

func LZ4_compressBound(leng int) int {
	return lz4.CompressBound(leng)
}

func LZ4_compress_default(src []byte, dst []byte) uint32 {
	n, err := lz4.Compress(dst, src)
	if err != nil {
		panic(err)
	}
	return uint32(n)
}

func LZ4_decompress_safe(src []byte) ([]byte, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("empty LZ4 input")
	}
	dst := make([]byte, len(src)*3)
	n, err := LZ4_decompress_fast(src, dst)
	for err != nil {
		dst = make([]byte, len(dst)*2)
		n, err = LZ4_decompress_fast(src, dst)
	}
	return dst[:n], err
}

func LZ4_decompress_fast(src []byte, dst []byte) (int, error) {
	return lz4.Decompress(dst, src)
}

const Letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
