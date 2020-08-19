// Package captain 定义了透传,各底层协议转换(数据报->数据流,数据流->数据报的转换)
package through

import (
	"io"

	"google.golang.org/protobuf/proto"

	"github.com/thinkgos/jocasta/core/captain"
	"github.com/thinkgos/jocasta/pkg/through/ddt"
)

// TVersion 透传协议版本
const TVersion = 1

// 透传节点类型
const (
	TTypesUnknown = iota
	TTypesClient
	TTypesServer
)

const (
	TRepSuccess            = iota // 成功
	TRepFailure                   // 失败
	TRepServerFailure             // 服务器问题
	TRepNetworkUnreachable        // 网络不可达
	TRepTTypesNotSupport          // 类型不支持
	TRepConnectionRefused         // 连接拒绝
)

// ThroughRequest through protocol request
// handshake request/response is formed as follows:
// +---------+-------+------------+----------+
// |  TYPES  |  VER  |  DATA_LEN  |   DATA   |
// +---------+-------+------------+----------+
// |    1    |   1   |    1 - 3   | Variable |
// +---------+-------+------------+----------+
// TTYPES 底三位为节点类型,
// VER 版本
// DATA_LEN see data length defined
// 数据
type ThroughRequest struct {
	Types   byte
	Version byte
	Data    []byte
}

// ParseRawThroughRequest parse to ThroughRequest
func ParseRawThroughRequest(r io.Reader) (msg ThroughRequest, err error) {
	// read message type,version
	tmp := []byte{0, 0}
	if _, err = io.ReadFull(r, tmp); err != nil {
		return
	}
	msg.Types, msg.Version = tmp[0]&0x07, tmp[1]

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
	msg.Data = data
	return
}

func (sf ThroughRequest) Bytes() ([]byte, error) {
	ds, n, err := captain.DataLen(len(sf.Data))
	if err != nil {
		return nil, err
	}
	bs := make([]byte, 0, n+len(sf.Data))
	bs = append(bs, sf.Types&0x07, sf.Version)
	bs = append(bs, ds[:n]...)
	bs = append(bs, sf.Data...)
	return bs, nil
}

type ThroughNegotiateRequest struct {
	Types   byte
	Version byte
	Nego    ddt.NegotiateRequest
}

func ParseThroughNegotiateRequest(r io.Reader) (*ThroughNegotiateRequest, error) {
	tr, err := ParseRawThroughRequest(r)
	if err != nil {
		return nil, err
	}

	tnr := &ThroughNegotiateRequest{
		Types:   tr.Types,
		Version: tr.Version,
	}

	err = proto.Unmarshal(tr.Data, &tnr.Nego)
	if err != nil {
		return nil, err
	}
	return tnr, nil
}

func (sf *ThroughNegotiateRequest) Bytes() ([]byte, error) {
	data, err := proto.Marshal(&sf.Nego)
	if err != nil {
		return nil, err
	}
	tr := ThroughRequest{
		Types:   sf.Types,
		Version: sf.Version,
		Data:    data,
	}
	return tr.Bytes()
}

// ThroughRequest through protocol reply
// handshake request/response is formed as follows:
// +---------+-------+
// |  STATUS |  VER  |
// +---------+-------+
// |    1    |   1   |
// +---------+-------+
// STATUS 状态
// VER 版本
type ThroughReply struct {
	Status  byte
	Version byte
}

// ParseRawThroughReply parse to ThroughReply
func ParseRawThroughReply(r io.Reader) (tr ThroughReply, err error) {
	// read message type,version
	tmp := []byte{0, 0}
	if _, err = io.ReadFull(r, tmp); err != nil {
		return
	}
	tr.Status, tr.Version = tmp[0], tmp[1]
	return
}
