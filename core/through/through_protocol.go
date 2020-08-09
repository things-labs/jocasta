package through

import (
	"net"
	"time"

	"github.com/thinkgos/jocasta/core/through/ddt"
	"github.com/thinkgos/jocasta/lib/extnet"
)

func WriteConnType(conn net.Conn, timeout time.Duration, connType uint8, data ...string) error {
	return extnet.WrapWriteTimeout(conn, timeout, func(c net.Conn) error {
		_, err := c.Write(ddt.BuildStringsWithType(connType, data...))
		return err
	})
}

func ReadConnType(conn net.Conn, timeout time.Duration, connType *uint8, data ...*string) error {
	return extnet.WrapReadTimeout(conn, timeout, func(c net.Conn) error {
		return ddt.ReadStringsWithType(c, connType, data...) // 连接类型和sk,节点id
	})
}

func WriteStrings(conn net.Conn, timeout time.Duration, data ...string) error {
	return extnet.WrapWriteTimeout(conn, timeout, func(c net.Conn) error {
		_, err := c.Write(ddt.BuildStrings(data...))
		return err
	})
}

func ReadStrings(conn net.Conn, timeout time.Duration, data ...*string) error {
	return extnet.WrapReadTimeout(conn, timeout, func(c net.Conn) error {
		return ddt.ReadStrings(conn, data...)
	})
}

func WriteUdp(conn net.Conn, timeout time.Duration, addr string, data []byte) error {
	return extnet.WrapWriteTimeout(conn, timeout, func(c net.Conn) error {
		_, err := c.Write(ddt.BuildUDPPacket(addr, data))
		return err
	})
}

func ReadUdp(conn net.Conn) (string, []byte, error) {
	return ddt.ReadUDPPacket(conn)
}
