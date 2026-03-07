package transport

import "io"

type Dialer interface {
    Dial(path string) (io.ReadWriteCloser, error)
}

func NewDialer() Dialer {
    return defaultDialer{}
}
