package cs

import (
	"fmt"
	"net"
	"runtime/debug"
	"strconv"

	"github.com/xtaci/kcp-go/v5"

	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/lib/gpool"
)

// KCP 传输,可选snappy压缩
type KCP struct {
	common
	l       net.Listener
	cfg     KcpConfig
	handler func(conn net.Conn)
	gPool   gpool.Pool
}

func NewKcp(addr string, cfg KcpConfig, handler func(conn net.Conn), opts ...KcpOption) (*KCP, error) {
	com, err := newCommon(addr)
	if err != nil {
		return nil, err
	}

	k := &KCP{
		common:  com,
		cfg:     cfg,
		handler: handler,
	}
	for _, opt := range opts {
		opt(k)
	}
	return k, nil
}

func (sf *KCP) ListenAndServe() error {
	lis, err := kcp.ListenWithOptions(net.JoinHostPort(sf.ip, strconv.Itoa(sf.port)), sf.cfg.Block, sf.cfg.DataShard, sf.cfg.ParityShard)
	if err != nil {
		sf.status <- err
		return err
	}
	defer lis.Close()

	if err = lis.SetDSCP(sf.cfg.DSCP); err != nil {
		sf.status <- fmt.Errorf("SetDSCP %+v", err)
		return err
	}
	if err = lis.SetReadBuffer(sf.cfg.SockBuf); err != nil {
		sf.status <- fmt.Errorf("SetReadBuffer %+v", err)
		return err
	}
	if err = lis.SetWriteBuffer(sf.cfg.SockBuf); err != nil {
		sf.status <- fmt.Errorf("SetWriteBuffer %+v", err)
		return err
	}

	sf.l = lis
	sf.status <- nil
	for {
		conn, err := lis.AcceptKCP()
		if err != nil {
			return err
		}
		sf.submit(func() {
			var c net.Conn

			conn.SetStreamMode(true)
			conn.SetWriteDelay(true)
			conn.SetNoDelay(sf.cfg.NoDelay, sf.cfg.Interval, sf.cfg.Resend, sf.cfg.NoCongestion)
			conn.SetMtu(sf.cfg.MTU)
			conn.SetWindowSize(sf.cfg.SndWnd, sf.cfg.RcvWnd)
			conn.SetACKNoDelay(sf.cfg.AckNodelay)

			if sf.cfg.NoComp {
				c = conn
			} else {
				c = csnappy.New(conn)
			}
			sf.handler(c)
		})
	}
}

// Close close kcp
func (sf *KCP) Close() (err error) {
	if sf.l != nil {
		err = sf.l.Close()
	}
	return
}

// Addr return address
func (sf *KCP) Addr() (addr string) {
	if sf.l != nil {
		addr = sf.l.Addr().String()
	}
	return
}

// 提交任务到协程
func (sf *KCP) submit(f func()) {
	fn := func() {
		defer func() {
			if err := recover(); err != nil {
				sf.log.Errorf("kcp connection handler crashed, %s , \ntrace:%s", err, string(debug.Stack()))
			}
		}()
		f()
	}
	if sf.gPool == nil || sf.gPool.Submit(fn) != nil {
		go fn()
	}
}

// KcpOption kcp option for kcp
type KcpOption func(*KCP)

// WithKcpGPool with gpool.Pool
func WithKcpGPool(pool gpool.Pool) KcpOption {
	return func(k *KCP) {
		if pool != nil {
			k.gPool = pool
		}
	}
}
