//go:generate stringer -type Types
// Package through 定义了透传,各底层协议转换(数据报->数据流,数据流->数据报的转换)
package through

import (
	"io"

	"github.com/thinkgos/jocasta/core/captain"
)

// Version 透传协议版本
const Version = 1

// Types 透传节点类型
type Types byte

// 透传节点类型
const (
	TypesUnknown Types = iota
	TypesClient
	TypesServer
)

const (
	RepSuccess            = iota // 成功
	RepFailure                   // 失败
	RepServerFailure             // 服务器问题
	RepNetworkUnreachable        // 网络不可达
	RepTypesNotSupport           // 节点类型不支持
	RepConnectionRefused         // 连接拒绝
)

// Request through protocol request
// handshake request/response is formed as follows:
// +---------+-------+------------+----------+
// |  TYPES  |  VER  |  DATA_LEN  |   DATA   |
// +---------+-------+------------+----------+
// |    1    |   1   |    1 - 3   | Variable |
// +---------+-------+------------+----------+
// TTYPES 低三位为节点类型,其它保留为0
// VER 版本, 透传版本
// DATA_LEN see package captain data length defined
// DATA 数据
type Request struct {
	Types   Types
	Version byte
	Data    []byte
}

// ParseRequest parse to Request
func ParseRequest(r io.Reader) (req Request, err error) {
	// read request type and version
	tmp := []byte{0, 0}
	if _, err = io.ReadFull(r, tmp); err != nil {
		return
	}
	req.Types, req.Version = Types(tmp[0]&0x07), tmp[1]

	// read remain data len
	var length int
	length, err = captain.ParseDataLen(r)
	if err != nil {
		return
	}

	// read data
	data := make([]byte, length)
	if _, err = io.ReadFull(r, data); err != nil {
		return
	}
	req.Data = data
	return
}

// Bytes request to bytes
func (sf Request) Bytes() ([]byte, error) {
	return sf.value(true)
}

// Header returns s slice of datagram header except data
func (sf Request) Header() ([]byte, error) {
	return sf.value(false)
}

func (sf Request) value(hasData bool) (bs []byte, err error) {
	ds, n, err := captain.DataLen(len(sf.Data))
	if err != nil {
		return nil, err
	}
	if hasData {
		bs = make([]byte, 0, 2+n+len(sf.Data))
	} else {
		bs = make([]byte, 0, 2+n)
	}
	bs = append(bs, byte(sf.Types)&0x07, sf.Version)
	bs = append(bs, ds[:n]...)
	if hasData {
		bs = append(bs, sf.Data...)
	}
	return bs, nil
}

// Reply through protocol reply
// handshake request/response is formed as follows:
// +---------+-------+
// |  STATUS |  VER  |
// +---------+-------+
// |    1    |   1   |
// +---------+-------+
// STATUS 状态
// VER 版本
type Reply struct {
	Status  byte
	Version byte
}

// ParseReply parse to Reply
func ParseReply(r io.Reader) (reply Reply, err error) {
	// read type and version
	tmp := []byte{0, 0}
	if _, err = io.ReadFull(r, tmp); err != nil {
		return
	}
	reply.Status, reply.Version = tmp[0], tmp[1]
	return
}

// SendReply send reply
func SendReply(conn io.Writer, status byte, version byte) (err error) {
	_, err = conn.Write([]byte{status, version})
	return
}
