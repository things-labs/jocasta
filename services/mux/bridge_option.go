package mux

import (
	"github.com/thinkgos/jocasta/lib/gpool"
	"github.com/thinkgos/jocasta/lib/logger"
)

type BridgeOption func(b *Bridge)

func WithBridgeLogger(l logger.Logger) BridgeOption {
	return func(b *Bridge) {
		if l != nil {
			b.log = l
		}
	}
}

func WithBridgeGPool(pool gpool.Pool) BridgeOption {
	return func(b *Bridge) {
		if pool != nil {
			b.gPool = pool
		}
	}
}
