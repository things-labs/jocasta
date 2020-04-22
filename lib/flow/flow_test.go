package flow

import (
	"bytes"
	"testing"

	"go.uber.org/atomic"
)

func TestFlow(t *testing.T) {
	var wc atomic.Uint64
	var rc atomic.Uint64
	var tc atomic.Uint64

	c := Flow{
		new(bytes.Buffer),
		&wc,
		&rc,
		&tc,
	}

	count := uint64(10240)
	_, _ = c.Write(make([]byte, count))
	_, _ = c.Read(make([]byte, count))
	if got := wc.Load(); got != count {
		t.Fatalf("write count want %d,but %d", count, got)
	}
	if got := rc.Load(); got != count {
		t.Fatalf("read count want %d,but %d", count, got)
	}
	if got := tc.Load(); got != count*2 {
		t.Fatalf("total count want %d,but %d", count, got)
	}
}
