//go:build linux

package main

import (
	"net"
	"syscall"
)

func applyTFO(dialer *net.Dialer) {
	dialer.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, 30, 1)
		})
	}
}
