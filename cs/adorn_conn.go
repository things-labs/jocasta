package cs

import (
	"net"

	"go.uber.org/atomic"

	"github.com/thinkgos/jocasta/connection/cflow"
	"github.com/thinkgos/jocasta/connection/ciol"
	"github.com/thinkgos/jocasta/connection/csnappy"
)

// AdornConn defines the conn decorate.
type AdornConn func(conn net.Conn) net.Conn

// AdornConnsChain defines a adornConn array.
// NOTE: 在conn read或write调用过程是在链上从后往前执行的,(类似栈,先进后执行,后进先执行),
//  所以统计类的应放在链头,也就是AfterChains的第一个,最靠近出口
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
