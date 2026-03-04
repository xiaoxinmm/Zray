//go:build !linux

package main

import "net"

func applyServerTFO(lc *net.ListenConfig) {}
