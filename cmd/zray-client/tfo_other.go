//go:build !linux

package main

import "net"

func applyTFO(dialer *net.Dialer) {
	// TFO not supported on this platform
}
