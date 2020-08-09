package mux

import (
	"github.com/thinkgos/jocasta/lib/gpool"
	"github.com/thinkgos/jocasta/lib/logger"
)

type ClientOption func(b *Client)

func WithClientLogger(l logger.Logger) ClientOption {
	return func(b *Client) {
		if l != nil {
			b.log = l
		}
	}
}

func WithClientGPool(pool gpool.Pool) ClientOption {
	return func(b *Client) {
		if pool != nil {
			b.gPool = pool
		}
	}
}
