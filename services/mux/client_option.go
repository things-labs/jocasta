package mux

import (
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/pkg/sword"
)

type ClientOption func(b *Client)

func WithClientLogger(l logger.Logger) ClientOption {
	return func(b *Client) {
		if l != nil {
			b.log = l
		}
	}
}

func WithClientGPool(pool sword.GoPool) ClientOption {
	return func(b *Client) {
		b.gPool = pool
	}
}
