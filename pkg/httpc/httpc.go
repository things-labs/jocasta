package httpc

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/thinkgos/jocasta/pkg/logger"

	"github.com/thinkgos/jocasta/connection/sni"
	"github.com/thinkgos/jocasta/core/basicAuth"
	"github.com/thinkgos/jocasta/pkg/outil"
)

type Request struct {
	RawHeader       []byte
	conn            net.Conn
	Host            string
	Method          string
	RawURL          string
	hostOrURL       string
	basicAuthCenter *basicAuth.Center
	log             logger.Logger
	IsSNI           bool
}

func New(inConn net.Conn, bufSize int, opts ...Option) (req Request, err error) {
	req = Request{conn: inConn}
	for _, opt := range opts {
		opt(&req)
	}

	n := 0
	buf := make([]byte, bufSize)
	n, err = inConn.Read(buf)
	if err != nil {
		inConn.Close()
		return
	}
	req.RawHeader = buf[:n]

	var serverName string
	//try sni
	serverName, err = sni.ServerNameFromBytes(req.RawHeader)
	if err != nil { // sni fail , try http
		index := bytes.IndexByte(req.RawHeader, '\n')
		if index == -1 {
			err = fmt.Errorf("http decoder data line err:%s", outil.SubStr(string(req.RawHeader), 0, 50))
			inConn.Close()
			return
		}
		// get method and url
		fmt.Sscanf(string(req.RawHeader[:index]), "%s%s", &req.Method, &req.hostOrURL)
		if req.Method == "" || req.hostOrURL == "" {
			err = fmt.Errorf("http decoder data err:%s", outil.SubStr(string(req.RawHeader), 0, 50))
			inConn.Close()
			return
		}
	} else { // sni success
		req.Method = "SNI"
		req.hostOrURL = "https://" + serverName + ":443"
		req.IsSNI = true
	}
	req.Method = strings.ToUpper(req.Method)

	if err = req.BasicAuth(); err != nil {
		return
	}

	var port string
	if req.IsHTTPS() {
		req.Host = req.hostOrURL
		port = "443"
	} else {
		var u *url.URL
		req.RawURL = req.getHTTPRawURL()
		u, err = url.Parse(req.RawURL)
		if err != nil {
			return
		}
		req.Host = u.Host
		port = "80"
	}

	if !strings.Contains(req.Host, ":") {
		req.Host = req.Host + ":" + port
	}
	return
}

func (sf *Request) IsHTTPS() bool {
	return sf.Method == "CONNECT"
}

func (sf *Request) HTTPSReply() (err error) {
	_, err = fmt.Fprint(sf.conn, "HTTP/1.1 200 Connection established\r\n\r\n")
	return
}

// format: Proxy-Authorization: Basic *
func (sf *Request) GetProxyAuthUserPassPair() (string, error) {
	authorization := sf.getHeader("Proxy-Authorization")

	authorization = strings.Trim(authorization, " \r\n\t")
	if authorization == "" {
		fmt.Fprintf(sf.conn, "HTTP/1.1 %s Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"\"\r\n\r\nProxy Authentication Required", "407")
		sf.conn.Close()
		return "", errors.New("require auth header data")
	}
	basic := strings.Fields(authorization)
	if len(basic) != 2 {
		sf.conn.Close()
		return "", fmt.Errorf("authorization data error,ERR:%s", authorization)
	}

	user, err := base64.StdEncoding.DecodeString(basic[1])
	if err != nil {
		sf.conn.Close()
		return "", fmt.Errorf("authorization data parse error,ERR:%s", err)
	}
	return string(user), nil
}

func (sf *Request) GetProxyAuthUserPass() (string, string, error) {
	userPass, err := sf.GetProxyAuthUserPassPair()
	if err != nil {
		return "", "", err
	}
	ups := strings.Split(userPass, ":")
	if len(ups) != 2 {
		return "", "", errors.New("invalid user password pair")
	}
	return ups[0], ups[1], nil
}

func (sf *Request) BasicAuth() error {
	if sf.basicAuthCenter != nil {
		userIP := strings.Split(sf.conn.RemoteAddr().String(), ":")
		localIP := strings.Split(sf.conn.LocalAddr().String(), ":")
		URL := ""
		if sf.IsHTTPS() {
			URL = "https://" + sf.Host
		} else {
			URL = sf.getHTTPRawURL()
		}
		user, err := sf.GetProxyAuthUserPassPair()
		if err != nil {
			return err
		}

		authOk := sf.basicAuthCenter.Verify(user, userIP[0], localIP[0], URL)
		if !authOk {
			fmt.Fprintf(sf.conn, "HTTP/1.1 %s Proxy Authentication Required\r\n\r\nProxy Authentication Required", "407")
			sf.conn.Close()
			return fmt.Errorf("basic auth fail")
		}
	}
	return nil
}

// getHTTPRawURL get http raw url
func (sf *Request) getHTTPRawURL() string {
	if !strings.HasPrefix(sf.hostOrURL, "/") {
		return sf.hostOrURL
	}
	if host := sf.getHeader("host"); host != "" {
		return fmt.Sprintf("http://%s%s", host, sf.hostOrURL)
	}
	return ""
}

// get header with key
func (sf *Request) getHeader(key string) string {
	key = strings.ToUpper(key)
	for _, line := range strings.Split(string(sf.RawHeader), "\r\n") {
		keyValue := strings.SplitN(strings.Trim(line, "\r\n "), ":", 2)
		if len(keyValue) == 2 &&
			key == strings.ToUpper(strings.Trim(keyValue[0], " ")) {
			return strings.Trim(keyValue[1], " ")
		}
	}
	return ""
}
