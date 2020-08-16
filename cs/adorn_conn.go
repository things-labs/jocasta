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

// AdornCsnappy snappy chain
func AdornCsnappy(compress bool) func(conn net.Conn) net.Conn {
	if compress {
		return func(conn net.Conn) net.Conn {
			return csnappy.New(conn)
		}
	}
	return func(conn net.Conn) net.Conn {
		return conn
	}
}

// AdornTls tls chain
func AdornTls(conf *tls.Config) func(conn net.Conn) net.Conn {
	return func(conn net.Conn) net.Conn {
		return tls.Client(conn, conf)
	}
}

// AdornCencrypt cencrypt chain
func AdornCencrypt(cip *encrypt.Cipher) func(conn net.Conn) net.Conn {
	return func(conn net.Conn) net.Conn {
		return cencrypt.New(conn, cip)
	}
}

// AdornCflow cflow chain
func AdornCflow(Wc *atomic.Uint64, Rc *atomic.Uint64, Tc *atomic.Uint64) func(conn net.Conn) net.Conn {
	return func(conn net.Conn) net.Conn {
		return &cflow.Conn{Conn: conn, Wc: Wc, Rc: Rc, Tc: Tc}
	}
}

// AdornCiol ciol chain
func AdornCiol(opts ...ciol.Options) func(conn net.Conn) net.Conn {
	return func(conn net.Conn) net.Conn {
		return ciol.New(conn, opts...)
	}
}
