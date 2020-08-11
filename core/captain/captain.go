package captain

import "errors"

var ErrUnrecognizedAddrType = errors.New("Unrecognized address type")

// address type defined
const (
	ATYPIPv4   = byte(0x01)
	ATYPDomain = byte(0x03)
	ATYPIPv6   = byte(0x04)
)
