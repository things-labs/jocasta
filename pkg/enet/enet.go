package enet

import (
	"bytes"
	"net"
	"strings"
	"time"

	"github.com/thinkgos/jocasta/internal/bytesconv"
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

var httpMethod = []string{
	"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH",
	"get", "head", "post", "put", "delete", "connect", "options", "trace", "patch",
}

// IsHTTP 是否是http请求
func IsHTTP(head []byte) bool {
	for _, method := range httpMethod {
		if len(head) >= len(method) && bytesconv.Bytes2Str(head[:len(method)]) == method {
			return true
		}
	}
	return false
}

// IsSocks5 是否是sockV5请求
func IsSocks5(head []byte) bool {
	return len(head) >= 3 &&
		head[0] == 0x05 &&
		(0 < head[1] && head[1] < 255) &&
		len(head) == 2+int(head[1])
}

func InsertProxyHeaders(head []byte, headers string) []byte {
	return bytes.Replace(head, []byte("\r\n"), []byte("\r\n"+headers), 1)
}

func RemoveProxyHeaders(head []byte) []byte {
	newLines := [][]byte{}
	keys := make(map[string]bool)
	lines := bytes.Split(head, []byte("\r\n"))
	IsBody := false
	i := -1
	for _, line := range lines {
		i++
		if len(line) == 0 || IsBody {
			newLines = append(newLines, line)
			IsBody = true
		} else {
			hline := bytes.SplitN(line, []byte(":"), 2)
			if i == 0 && IsHTTP(head) {
				newLines = append(newLines, line)
				continue
			}
			if len(hline) != 2 {
				continue
			}
			k := strings.ToUpper(string(hline[0]))
			if _, ok := keys[k]; ok || strings.HasPrefix(k, "PROXY-") {
				continue
			}
			keys[k] = true
			newLines = append(newLines, line)
		}
	}
	return bytes.Join(newLines, []byte("\r\n"))
}
