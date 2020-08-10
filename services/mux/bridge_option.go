package mux

import (
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/pkg/sword"
)

type BridgeOption func(b *Bridge)

func WithBridgeLogger(l logger.Logger) BridgeOption {
	return func(b *Bridge) {
		if l != nil {
			b.log = l
		}
	}
}

func WithBridgeGPool(pool sword.GoPool) BridgeOption {
	return func(b *Bridge) {
		b.gPool = pool
	}
}
