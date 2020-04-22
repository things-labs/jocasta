// Provides leaky buffer, based on the example in Effective Go.
// set https://studygolang.com/articles/1976
package bpool

type BufferPool interface {
	Get() []byte
	Put([]byte)
}
