//go:build windows

package transport

import (
    "io"
    "os"
)

type defaultDialer struct{}

func (defaultDialer) Dial(path string) (io.ReadWriteCloser, error) {
    return os.OpenFile(path, os.O_RDWR, 0)
}
