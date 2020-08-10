package mux

import (
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/pkg/sword"
)

type ServerOption func(b *Server)

func WithServerLogger(l logger.Logger) ServerOption {
	return func(b *Server) {
		if l != nil {
			b.log = l
		}
	}
}

func WithServerGPool(pool sword.GoPool) ServerOption {
	return func(b *Server) {
		b.gPool = pool
	}
}
