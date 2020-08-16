package cs

import (
	"crypto/tls"
	"net"

	"go.uber.org/atomic"

	"github.com/thinkgos/jocasta/connection/cencrypt"
	"github.com/thinkgos/jocasta/connection/cflow"
	"github.com/thinkgos/jocasta/connection/ciol"
	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/lib/encrypt"
)

// AdornConn defines the conn decorate.
type AdornConn func(conn net.Conn) net.Conn

// HandlersChain defines a HandlerFunc array.
type AdornConnsChain []AdornConn

// ChainCsnappy snappy chain
func ChainCsnappy(compress bool) func(conn net.Conn) net.Conn {
	if compress {
		return func(conn net.Conn) net.Conn {
			return csnappy.New(conn)
		}
	}
	return func(conn net.Conn) net.Conn {
		return conn
	}
}

// ChainTls tls chain
func ChainTls(conf *tls.Config) func(conn net.Conn) net.Conn {
	return func(conn net.Conn) net.Conn {
		return tls.Client(conn, conf)
	}
}

// ChainCencrypt cencrypt chain
func ChainCencrypt(cip *encrypt.Cipher) func(conn net.Conn) net.Conn {
	return func(conn net.Conn) net.Conn {
		return cencrypt.New(conn, cip)
	}
}

// ChainCflow cflow chain
func ChainCflow(Wc *atomic.Uint64, Rc *atomic.Uint64, Tc *atomic.Uint64) func(conn net.Conn) net.Conn {
	return func(conn net.Conn) net.Conn {
		return &cflow.Conn{Conn: conn, Wc: Wc, Rc: Rc, Tc: Tc}
	}
}

// ChainCiol ciol chain
func ChainCiol(opts ...ciol.Options) func(conn net.Conn) net.Conn {
	return func(conn net.Conn) net.Conn {
		return ciol.New(conn, opts...)
	}
}
