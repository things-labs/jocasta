package bpool

import (
	"sync"
	"testing"
)

func TestBuffer(t *testing.T) {
	p := NewBuffer(2048, 2048)
	b := p.Get()
	bs := b[0:cap(b)]
	if len(bs) != cap(b) {
		t.Fatalf("invalid buffer")
	}
	p.Put(b)
}

func TestSyncPool(t *testing.T) {
	p := NewPool(2048)
	b := p.Get()
	bs := b[0:cap(b)]
	if len(bs) != cap(b) {
		t.Fatalf("invalid buffer")
	}
	p.Put(b)
}

func BenchmarkBuffer(b *testing.B) {
	p := NewBuffer(2048, 2048)
	wg := new(sync.WaitGroup)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		wg.Add(1)
		go func() {
			s := p.Get()
			p.Put(s)
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkSyncPool(b *testing.B) {
	p := NewPool(2048)
	wg := new(sync.WaitGroup)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		wg.Add(1)
		go func() {
			s := p.Get()
			p.Put(s)
			wg.Done()
		}()
	}
	wg.Wait()
}
