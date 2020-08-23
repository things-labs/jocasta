//go:generate stringer -type Types
// Package through 定义了透传,各底层协议转换(数据报->数据流,数据流->数据报的转换)
package through

import (
	"io"

	"google.golang.org/protobuf/proto"

	"github.com/thinkgos/jocasta/core/captain"
	"github.com/thinkgos/jocasta/pkg/through/ddt"
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

// NegotiateRequest negotiate request
type NegotiateRequest struct {
	Types   Types
	Version byte
	Nego    ddt.NegotiateRequest
}

// ParseNegotiateRequest parse negotiate request
func ParseNegotiateRequest(r io.Reader) (*NegotiateRequest, error) {
	tr, err := captain.ParseRequest(r)
	if err != nil {
		return nil, err
	}

	tnr := &NegotiateRequest{
		Types:   Types(tr.Types),
		Version: tr.Version,
	}
	if err = proto.Unmarshal(tr.Data, &tnr.Nego); err != nil {
		return nil, err
	}
	return tnr, nil
}

// Bytes to byte
func (sf *NegotiateRequest) Bytes() ([]byte, error) {
	data, err := proto.Marshal(&sf.Nego)
	if err != nil {
		return nil, err
	}
	tr := captain.Request{
		Types:   byte(sf.Types),
		Version: sf.Version,
		Data:    data,
	}
	return tr.Bytes()
}
