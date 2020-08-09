package httpc

import (
	"github.com/thinkgos/jocasta/lib/logger"

	"github.com/thinkgos/jocasta/core/basicAuth"
)

type Option func(r *Request)

func WithBasicAuth(center *basicAuth.Center) Option {
	return func(r *Request) {
		r.basicAuthCenter = center
	}
}

func WithHeader(header []byte) Option {
	return func(r *Request) {
		r.RawHeader = header
	}
}

func WithLogger(log logger.Logger) Option {
	return func(r *Request) {
		r.log = log
	}
}
