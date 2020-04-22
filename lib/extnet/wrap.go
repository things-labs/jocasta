package extnet

import (
	"net"
	"time"
)

func WrapWriteTimeout(conn net.Conn, timeout time.Duration, f func(c net.Conn) error) error {
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	err := f(conn)
	_ = conn.SetWriteDeadline(time.Time{})
	return err
}

func WrapReadTimeout(conn net.Conn, timeout time.Duration, f func(c net.Conn) error) error {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	err := f(conn)
	_ = conn.SetReadDeadline(time.Time{})
	return err
}

func WrapTimeout(conn net.Conn, timeout time.Duration, f func(c net.Conn) error) error {
	_ = conn.SetDeadline(time.Now().Add(timeout))
	err := f(conn)
	_ = conn.SetDeadline(time.Time{})
	return err
}
