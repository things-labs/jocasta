package cs

import "io"

type Channel interface {
	io.Closer
	LocalAddr() string
	ListenAndServe() error
}
