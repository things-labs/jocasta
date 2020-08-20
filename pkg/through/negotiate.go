package through

import (
	"io"

	"google.golang.org/protobuf/proto"

	"github.com/thinkgos/jocasta/pkg/through/ddt"
)

// NegotiateRequest negotiate request
type NegotiateRequest struct {
	Types   Types
	Version byte
	Nego    ddt.NegotiateRequest
}

// ParseNegotiateRequest parse negotiate request
func ParseNegotiateRequest(r io.Reader) (*NegotiateRequest, error) {
	tr, err := ParseRequest(r)
	if err != nil {
		return nil, err
	}

	tnr := &NegotiateRequest{
		Types:   tr.Types,
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
	tr := Request{
		Types:   sf.Types,
		Version: sf.Version,
		Data:    data,
	}
	return tr.Bytes()
}
