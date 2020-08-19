package captain

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"

	"github.com/thinkgos/jocasta/internal/bytesconv"
)

// Datagram udp datagram
// UDP request/response is formed as follows:
// +------+-------+----------+----------+----------+
// |  RSV |  ATYP |   ADDR   |   PORT   |   DATA   |
// +------+-------+----------+----------+----------+
// |  1   | X'00' | Variable |     2    | Variable |
// +------+-------+----------+----------+----------+
type Datagram struct {
	// Reserved byte
	Reserved byte
	// Addr address
	Addr AddrSpec
	// Data real data
	Data []byte
}

// NewDatagram new datagram with dest address and data
func NewDatagram(destAddr string, data []byte) (da Datagram, err error) {
	da.Addr, err = ParseAddrSpec(destAddr)
	if err != nil {
		return
	}
	if da.Addr.AddrType == ATYPDomain && len(da.Addr.FQDN) > math.MaxUint8 {
		err = errors.New("destination host name too long")
		return
	}
	da.Reserved, da.Data = 0, data
	return
}

// ParseDatagram parse to datagram from bytes
func ParseDatagram(b []byte) (da Datagram, err error) {
	if len(b) < 2+net.IPv4len+2 { // no enough data
		err = errors.New("datagram to short")
		return
	}
	// ignore RSV And get Address  type
	da.Reserved, da.Addr.AddrType = b[0], b[1]

	headLen := 2
	switch da.Addr.AddrType {
	case ATYPIPv4:
		headLen += net.IPv4len + 2
		da.Addr.IP = net.IPv4(b[2], b[3], b[4], b[5])
		da.Addr.Port = int(binary.BigEndian.Uint16((b[headLen-2:])))
	case ATYPIPv6:
		headLen += net.IPv6len + 2
		if len(b) <= headLen {
			err = errors.New("datagram to short")
			return
		}

		da.Addr.IP = b[2 : 2+net.IPv6len]
		da.Addr.Port = int(binary.BigEndian.Uint16(b[headLen-2:]))
	case ATYPDomain:
		addrLen := int(b[2])
		headLen += 1 + addrLen + 2
		if len(b) <= headLen {
			err = errors.New("datagram to short")
			return
		}
		da.Addr.FQDN = string(b[3 : 3+addrLen])
		da.Addr.Port = int(binary.BigEndian.Uint16(b[headLen-2:]))
	default:
		err = ErrUnrecognizedAddrType
		return
	}
	da.Data = b[headLen:]
	return
}

// Header returns s slice of datagram header except data
func (sf *Datagram) Header() []byte {
	return sf.values(false)
}

// Bytes datagram to bytes
func (sf *Datagram) Bytes() []byte {
	return sf.values(true)
}

func (sf *Datagram) values(hasData bool) (bs []byte) {
	var addr []byte

	length := 4
	switch sf.Addr.AddrType {
	case ATYPIPv4:
		length += net.IPv4len
		addr = sf.Addr.IP.To4()
	case ATYPIPv6:
		length += net.IPv6len
		addr = sf.Addr.IP.To16()
	case ATYPDomain:
		length += 1 + len(sf.Addr.FQDN)
		addr = bytesconv.Str2Bytes(sf.Addr.FQDN)
	default:
		panic(fmt.Sprintf("invalid address type: %d", sf.Addr.AddrType))
	}
	if hasData {
		bs = make([]byte, 0, length+len(sf.Data))
	} else {
		bs = make([]byte, 0, length)
	}

	bs = append(bs, sf.Reserved, sf.Addr.AddrType)
	if sf.Addr.AddrType == ATYPDomain {
		bs = append(bs, byte(len(sf.Addr.FQDN)))
	}
	bs = append(bs, addr...)
	bs = append(bs, byte(sf.Addr.Port>>8), byte(sf.Addr.Port))
	if hasData {
		bs = append(bs, sf.Data...)
	}
	return
}
