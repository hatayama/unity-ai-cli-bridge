//go:build !windows

package transport

import (
	"io"
	"net"
)

type defaultDialer struct{}

func (defaultDialer) Dial(path string) (io.ReadWriteCloser, error) {
	return net.Dial("unix", path)
}
