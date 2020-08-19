package captain

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"

	"github.com/thinkgos/jocasta/internal/bytesconv"
)

// StreamDatagram udp datagram transfer in stream
// UDP request/response is formed as follows:
// +------+-------+----------+----------+----------+----------+
// |  RSV |  ATYP |   ADDR   |   PORT   | DATA_LEN |   DATA   |
// +------+-------+----------+----------+----------+----------+
// |  1   | X'00' | Variable |     2    | Variable | Variable |
// +------+-------+----------+----------+----------+----------+
type StreamDatagram struct {
	// Reserved byte
	Reserved byte
	// Addr address
	Addr AddrSpec
	// Data real data
	Data []byte
}

// NewStreamDatagram new stream datagram with dest address and data
func NewStreamDatagram(destAddr string, data []byte) (da StreamDatagram, err error) {
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

// ParseStreamDatagram parse datagram from stream
func ParseStreamDatagram(r io.Reader) (da Datagram, err error) {
	tmp := []byte{0, 0}
	// ignore RSV and get Address type
	if _, err = io.ReadFull(r, tmp); err != nil {
		return
	}
	da.Reserved, da.Addr.AddrType = tmp[0], tmp[1]

	switch da.Addr.AddrType {
	case ATYPIPv4:
		ip := make([]byte, net.IPv4len+2)
		// get IPv4 and port
		if _, err = io.ReadFull(r, ip); err != nil {
			return
		}
		da.Addr.IP = net.IPv4(ip[0], ip[1], ip[2], ip[3])
		da.Addr.Port = int(binary.BigEndian.Uint16((ip[net.IPv4len:])))
	case ATYPIPv6:
		ip := make([]byte, net.IPv6len+2)
		// get IPv6 and port
		if _, err = io.ReadFull(r, ip); err != nil {
			return
		}
		da.Addr.IP = ip[:net.IPv6len]
		da.Addr.Port = int(binary.BigEndian.Uint16(ip[net.IPv6len:]))
	case ATYPDomain:
		if _, err = io.ReadFull(r, tmp[:1]); err != nil {
			return
		}
		addrLen := int(tmp[0])
		fqdn := make([]byte, addrLen+2)
		// get FQDN and port
		if _, err = io.ReadFull(r, fqdn); err != nil {
			return
		}
		da.Addr.FQDN = string(fqdn[:addrLen])
		da.Addr.Port = int(binary.BigEndian.Uint16(fqdn[addrLen:]))
	default:
		err = ErrUnrecognizedAddrType
		return
	}

	// data len
	var length int
	length, err = ParseDataLen(r)
	if err != nil {
		return
	}
	data := make([]byte, length)
	if _, err = io.ReadFull(r, data); err != nil {
		return
	}
	da.Data = data
	return
}

// Header returns s slice of datagram header except data
func (sf *StreamDatagram) Header() ([]byte, error) {
	return sf.values(false)
}

// Bytes datagram to bytes
func (sf *StreamDatagram) Bytes() ([]byte, error) {
	return sf.values(true)
}

func (sf *StreamDatagram) values(hasData bool) ([]byte, error) {
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
		return nil, fmt.Errorf("invalid address type: %d", sf.Addr.AddrType)
	}

	ds, n, err := DataLen(len(sf.Data))
	if err != nil {
		return nil, err
	}
	length += n

	var bs []byte
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
	bs = append(bs, ds[:n]...)
	if hasData {
		bs = append(bs, sf.Data...)
	}
	return bs, nil
}
