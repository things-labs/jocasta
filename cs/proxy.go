package cs

import (
	"errors"
	"net/url"
	"strings"

	"golang.org/x/net/proxy"
)

// ValidProxyURL 校验proxyURL是否正确
func ValidProxyURL(proxyURL string) bool {
	_, err := ParseProxyURL(proxyURL)
	return err == nil
}

// ParseProxyURL parse proxy url
// proxyURL格式如下:
// 		https://username:password@host:port
// 		https://host:port
// 		socks5://username:password@host:port
// 		socks5://host:port
func ParseProxyURL(proxyURL string) (*url.URL, error) {
	if strings.HasPrefix(proxyURL, "socks5://") ||
		strings.HasPrefix(proxyURL, "https://") {
		return url.Parse(proxyURL)
	}
	return nil, errors.New("invalid proxy url")
}

func ProxyAuth(proxyURL *url.URL) (auth *proxy.Auth) {
	if proxyURL != nil && proxyURL.User != nil {
		pwd, _ := proxyURL.User.Password()
		auth = &proxy.Auth{
			User:     proxyURL.User.Username(),
			Password: pwd,
		}
	}
	return
}
