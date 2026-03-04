//go:build linux

package main

import (
	"net"
	"syscall"
)

func applyServerTFO(lc *net.ListenConfig) {
	lc.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, 23, 4096)
		})
	}
}
