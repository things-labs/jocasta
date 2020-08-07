package extnet

import (
	"net"
	"time"
)

// WrapWriteTimeout wrap function with SetWriteDeadLine
func WrapWriteTimeout(conn net.Conn, timeout time.Duration, f func(c net.Conn) error) error {
	conn.SetWriteDeadline(time.Now().Add(timeout)) // nolint: errcheck
	err := f(conn)
	conn.SetWriteDeadline(time.Time{}) // nolint: errcheck
	return err
}

// WrapReadTimeout wrap function with SetReadDeadline
func WrapReadTimeout(conn net.Conn, timeout time.Duration, f func(c net.Conn) error) error {
	conn.SetReadDeadline(time.Now().Add(timeout)) // nolint: errcheck
	err := f(conn)
	conn.SetReadDeadline(time.Time{}) // nolint: errcheck
	return err
}

// WrapTimeout wrap function with SetDeadline
func WrapTimeout(conn net.Conn, timeout time.Duration, f func(c net.Conn) error) error {
	conn.SetDeadline(time.Now().Add(timeout)) // nolint: errcheck
	err := f(conn)
	conn.SetDeadline(time.Time{}) // nolint: errcheck
	return err
}
