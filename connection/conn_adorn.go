// Copyright [2020] [thinkgos] thinkgo@aliyun.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connection

import (
	"compress/gzip"
	"compress/zlib"
	"crypto/tls"
	"net"

	"github.com/things-go/encrypt"
	"go.uber.org/atomic"

	"github.com/thinkgos/jocasta/connection/cencrypt"
	"github.com/thinkgos/jocasta/connection/cflow"
	"github.com/thinkgos/jocasta/connection/cgzip"
	"github.com/thinkgos/jocasta/connection/ciol"
	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/connection/czlib"
)

// BaseAdornTLSClient base adorn tls client
func BaseAdornTLSClient(conf *tls.Config) AdornConn {
	return func(conn net.Conn) net.Conn {
		return tls.Client(conn, conf)
	}
}

// BaseAdornTLSServer base adorn tls server
func BaseAdornTLSServer(conf *tls.Config) AdornConn {
	return func(conn net.Conn) net.Conn {
		return tls.Server(conn, conf)
	}
}

// BaseAdornStcp base adorn encrypt with method and password
func BaseAdornStcp(method, password string) AdornConn {
	return func(conn net.Conn) net.Conn {
		cip, err := encrypt.NewCipher(method, password)
		if err != nil {
			panic("encrypt method should be valid")
		}
		return cencrypt.New(conn, cip)
	}
}

// AdornSnappy snappy chain
func AdornSnappy(compress bool) AdornConn {
	if compress {
		return func(conn net.Conn) net.Conn {
			return csnappy.New(conn)
		}
	}
	return func(conn net.Conn) net.Conn {
		return conn
	}
}

// AdornGzip gzip chain
func AdornGzip(compress bool) AdornConn {
	return AdornGzipLevel(compress, gzip.DefaultCompression)
}

// AdornGzipLevel gzip chain with level
// level see gzip package
func AdornGzipLevel(compress bool, level int) AdornConn {
	if compress {
		return func(conn net.Conn) net.Conn {
			return cgzip.NewLevel(conn, level)
		}
	}
	return func(conn net.Conn) net.Conn {
		return conn
	}
}

// AdornZlib zlib chain
func AdornZlib(compress bool) AdornConn {
	return AdornZlibLevel(compress, zlib.DefaultCompression)
}

// AdornZlibLevel zlib chain with the level
// level see zlib package
func AdornZlibLevel(compress bool, level int) AdornConn {
	return AdornZlibLevelDict(compress, level, nil)
}

// AdornZlibLevelDict zlib chain with the level and dict
// level see zlib package
func AdornZlibLevelDict(compress bool, level int, dict []byte) AdornConn {
	return func(conn net.Conn) net.Conn {
		if compress {
			return czlib.NewLevelDict(conn, level, dict)
		}
		return conn
	}
}

// AdornFlow cflow chain
func AdornFlow(wc, rc, tc *atomic.Uint64) AdornConn {
	return func(conn net.Conn) net.Conn {
		return &cflow.Conn{Conn: conn, Wc: wc, Rc: rc, Tc: tc}
	}
}

// AdornIol ciol chain
func AdornIol(opts ...ciol.Options) AdornConn {
	return func(conn net.Conn) net.Conn {
		return ciol.New(conn, opts...)
	}
}
