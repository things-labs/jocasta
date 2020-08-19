package through

import (
	"io"

	"github.com/thinkgos/jocasta/pkg/through/ddt"
	"google.golang.org/protobuf/proto"
)

// Request through protocol request
// handshake request/response is formed as follows:
// +-------+------------+----------+
// |  VER  |  DATA_LEN  |   DATA   |
// +-------+------------+----------+
// |   1   |    1 - 3   | Variable |
// +-------+------------+----------+
// VER 版本, 透传版本
// DATA_LEN see package captain data length defined
// DATA 数据
type HandshakeRequest struct {
	Version byte
	Hand    ddt.HandshakeRequest
}

// ParseHandshakeRequest parse negotiate request
func ParseHandshakeRequest(r io.Reader) (*HandshakeRequest, error) {
	tr, err := ParseRequest(r)
	if err != nil {
		return nil, err
	}

	tnr := &HandshakeRequest{
		Version: tr.Version,
	}
	if err = proto.Unmarshal(tr.Data, &tnr.Hand); err != nil {
		return nil, err
	}
	return tnr, nil
}

// Bytes to byte
func (sf *HandshakeRequest) Bytes() ([]byte, error) {
	data, err := proto.Marshal(&sf.Hand)
	if err != nil {
		return nil, err
	}
	tr := Request{
		Version: sf.Version,
		Data:    data,
	}
	return tr.Bytes()
}
