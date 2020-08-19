package extnet

import (
	"bytes"
	"net"
	"strconv"
	"strings"

	"github.com/thinkgos/jocasta/internal/bytesconv"
)

// IsErrClosed is error closed
func IsErrClosed(err error) bool {
	return err != nil && strings.Contains(err.Error(), "use of closed network connection")
}

// IsErrTimeout is net error timeout
func IsErrTimeout(err error) bool {
	if err == nil {
		return false
	}
	e, ok := err.(net.Error)
	return ok && e.Timeout()
}

// IsErrRefused is error connection refused
func IsErrRefused(err error) bool {
	return err != nil && strings.Contains(err.Error(), "connection refused")
}

// IsErrDeadline is error i/o deadline reached
func IsErrDeadline(err error) bool {
	return err != nil && strings.Contains(err.Error(), "i/o deadline reached")
}

// IsErrSocketNotConnected is error socket is not connected
func IsErrSocketNotConnected(err error) bool {
	return err != nil && strings.Contains(err.Error(), "socket is not connected")
}

func SplitHostPort(addr string) (string, uint16, error) {
	host, p, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.ParseUint(p, 10, 16)
	if err != nil {
		return "", 0, err
	}
	return host, uint16(port), nil
}

// IsDomain 是否是域名,只检查host或ip,不可带port,否则会误判
func IsDomain(host string) bool {
	return net.ParseIP(host) == nil
}

// IsIntranet is intranet network,if host is domain,it will looks up host using the local resolver.
// net.LookupIP may cause deadlock in windows
// see https://github.com/golang/go/issues/24178
// 局域网IP段:
// 		A类: 10.0.0.0~10.255.255.255
// 		B类: 172.16.0.0~172.31.255.255
// 		C类: 192.168.0.0~192.168.255.255
func IsIntranet(host string) bool {
	var ips []net.IP
	var err error

	if _ip := net.ParseIP(host); _ip != nil { // is ip
		ips = []net.IP{_ip}
	} else if ips, err = net.LookupIP(host); err != nil { // is domain
		return false
	}

	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil &&
			(ip4[0] == 127 || // ipv4 loopback
				ip4[0] == 10 || // A类
				(ip4[0] == 172 && (ip4[1] >= 16) && (ip4[1] <= 31)) || // B类
				(ip4[0] == 192 && ip4[1] == 168)) || // C类
			ip.Equal(net.IPv6loopback) { // ipv6 loopback
			return true
		}
	}
	return false
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
