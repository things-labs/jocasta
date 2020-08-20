package mux

import (
	"github.com/thinkgos/jocasta/lib/logger"
)

type ServerOption func(b *Server)

func WithServerLogger(l logger.Logger) ServerOption {
	return func(b *Server) {
		if l != nil {
			b.log = l
		}
	}
}
