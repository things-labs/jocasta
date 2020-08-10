// Package captain 定义了透传,各底层协议转换(数据报->数据流,数据流->数据报的转换)
package captain

import (
	"io"

	"google.golang.org/protobuf/proto"

	"github.com/thinkgos/jocasta/pkg/captain/ddt"
)

// TVersion 透传协议版本
const TVersion = 1

// 透传节点类型
const (
	TTypesUnknown = iota
	TTypesClient
	TTypesServer
)

// Through through protocol message
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
type Through struct {
	Types   byte
	Version byte
	Data    []byte
}

// ParseRawThrough parse to Through
func ParseRawThrough(r io.Reader) (msg Through, err error) {
	// read message type,version
	tmp := []byte{0, 0}
	if _, err = io.ReadFull(r, tmp); err != nil {
		return
	}
	msg.Types, msg.Version = tmp[0], tmp[1]

	// read remain data len
	var length int
	length, err = ParseDataLen(r)
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

func (sf Through) Bytes() ([]byte, error) {
	ds, n, err := DataLen(len(sf.Data))
	if err != nil {
		return nil, err
	}
	bs := make([]byte, 0, n+len(sf.Data))
	bs = append(bs, sf.Types, sf.Version)
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
	tr, err := ParseRawThrough(r)
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
	tr := Through{
		Types:   sf.Types,
		Version: sf.Version,
		Data:    data,
	}
	return tr.Bytes()
}
