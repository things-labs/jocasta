package extnet

import (
	"bytes"
	"net"
	"strconv"
	"strings"
)

func IsErrClosed(err error) bool {
	return err != nil && strings.Contains(err.Error(), "use of closed network connection")
}

func IsErrTimeout(err error) bool {
	if err == nil {
		return false
	}
	e, ok := err.(net.Error)
	return ok && e.Timeout()
}

func IsErrRefused(err error) bool {
	return err != nil && strings.Contains(err.Error(), "connection refused")
}

func IsErrDeadline(err error) bool {
	return err != nil && strings.Contains(err.Error(), "i/o deadline reached")
}

func IsErrSocketNotConnected(err error) bool {
	return err != nil && strings.Contains(err.Error(), "socket is not connected")
}

// IsDomain 是否是域名
func IsDomain(address string) bool {
	return net.ParseIP(address) == nil
}

/*
net.LookupIP may cause  deadlock in windows
https://github.com/golang/go/issues/24178
*/
func IsInternalIP(address string) bool {
	var outIPs []net.IP
	var err error

	if IsDomain(address) {
		if outIPs, err = net.LookupIP(address); err != nil {
			return false
		}
	} else {
		outIPs = []net.IP{net.ParseIP(address)}
	}

	for _, ip := range outIPs {
		if ip.IsLoopback() {
			return true
		}
		if ip.To4().Mask(net.IPv4Mask(255, 0, 0, 0)).String() == "10.0.0.0" {
			return true
		}
		if ip.To4().Mask(net.IPv4Mask(255, 255, 0, 0)).String() == "192.168.0.0" {
			return true
		}
		if ip.To4().Mask(net.IPv4Mask(255, 0, 0, 0)).String() == "172.0.0.0" {
			i, _ := strconv.Atoi(strings.Split(ip.To4().String(), ".")[1])
			return i >= 16 && i <= 31
		}
	}
	return false
}

// IsHTTP 是否是http请求
func IsHTTP(head []byte) bool {
	keys := []string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}
	for _, key := range keys {
		if bytes.HasPrefix(head, []byte(key)) || bytes.HasPrefix(head, []byte(strings.ToLower(key))) {
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
