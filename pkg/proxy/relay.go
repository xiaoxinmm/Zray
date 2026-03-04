// Package proxy implements bidirectional stream copying with stats.
package proxy

import (
	"io"
	"sync"
	"sync/atomic"
)

var bufPool = sync.Pool{New: func() interface{} { return make([]byte, 32*1024) }}

// Relay copies data bidirectionally between a and b.
// Returns total bytes uploaded (a->b) and downloaded (b->a).
func Relay(a, b io.ReadWriter) (up, down int64) {
	var u, d int64
	done := make(chan struct{}, 2)
	go func() {
		d = copyWithCount(a, b)
		done <- struct{}{}
	}()
	go func() {
		u = copyWithCount(b, a)
		done <- struct{}{}
	}()
	<-done
	return u, d
}

func copyWithCount(dst io.Writer, src io.Reader) int64 {
	buf := bufPool.Get().([]byte)
	defer bufPool.Put(buf)
	var total int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			wn, werr := dst.Write(buf[:n])
			total += int64(wn)
			if werr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
	return total
}

// Counter is an atomic byte counter for stats.
type Counter struct {
	bytes int64
}

func (c *Counter) Add(n int64) { atomic.AddInt64(&c.bytes, n) }
func (c *Counter) Load() int64 { return atomic.LoadInt64(&c.bytes) }
func (c *Counter) Reset() int64 {
	return atomic.SwapInt64(&c.bytes, 0)
}
