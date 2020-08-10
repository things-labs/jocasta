package through

import (
	"net"
	"time"

	"github.com/thinkgos/jocasta/lib/extnet"
)

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
