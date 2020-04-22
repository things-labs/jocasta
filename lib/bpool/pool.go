package bpool

import (
	"sync"
)

type Pool struct {
	size int
	pool *sync.Pool
}

func NewPool(size int) *Pool {
	return &Pool{
		size,
		&sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, size)
			}},
	}
}
func (sf *Pool) Get() []byte {
	return sf.pool.Get().([]byte)
}

func (sf *Pool) Put(b []byte) {
	if cap(b) != sf.size {
		panic("invalid buffer size that's put into leaky buffer")
	}
	sf.pool.Put(b[:0])
}
