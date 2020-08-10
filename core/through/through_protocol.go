package through

import (
	"net"
	"time"

	"github.com/thinkgos/jocasta/lib/extnet"
)

func WriteConnType(conn net.Conn, timeout time.Duration, connType uint8, data ...string) error {
	return extnet.WrapWriteTimeout(conn, timeout, func(c net.Conn) error {
		_, err := c.Write(BuildStringsWithType(connType, data...))
		return err
	})
}

func ReadConnType(conn net.Conn, timeout time.Duration, connType *uint8, data ...*string) error {
	return extnet.WrapReadTimeout(conn, timeout, func(c net.Conn) error {
		return ReadStringsWithType(c, connType, data...) // 连接类型和sk,节点id
	})
}

func WriteStrings(conn net.Conn, timeout time.Duration, data ...string) error {
	return extnet.WrapWriteTimeout(conn, timeout, func(c net.Conn) error {
		_, err := c.Write(BuildString(data...))
		return err
	})
}

func ReadStrings(conn net.Conn, timeout time.Duration, data ...*string) error {
	return extnet.WrapReadTimeout(conn, timeout, func(c net.Conn) error {
		return ReadString(conn, data...)
	})
}
