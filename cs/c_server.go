package cs

import "io"

type Channel interface {
	io.Closer
	Addr() string
	Status() <-chan error
	ListenAndServe() error
}
