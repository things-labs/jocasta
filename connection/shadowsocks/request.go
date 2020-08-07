package shadowsocks

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
)

// 1(addrType) + [4/16/(1+[255 max])] + 2(port) + 10(hmac-sha1)
// AddrMask address type mask
const AddrMask byte = 0x0f

// address type
const (
	typeIPv4   = 1 // type is ipv4 address
	typeDomain = 3 // type is domain address
	typeIPv6   = 4 // type is ipv6 address
)

// ParseRequest parse request from Conn,get addr like host:port
func ParseRequest(r io.Reader) (addr string, err error) {
	var port uint16

	tmp := []byte{0}
	// read address type
	if _, err = io.ReadFull(r, tmp); err != nil {
		return
	}
	addrType := tmp[0]

	switch addrType & AddrMask {
	case typeIPv4:
		tmpAddr := make([]byte, net.IPv4len+2)
		if _, err = io.ReadFull(r, tmpAddr); err != nil {
			return
		}
		addr = net.IP(tmpAddr[:net.IPv4len]).String() // TODO: BUG??
		port = binary.BigEndian.Uint16(tmpAddr[net.IPv4len:])
	case typeIPv6:
		tmpAddr := make([]byte, net.IPv6len+2)
		if _, err = io.ReadFull(r, tmpAddr); err != nil {
			return
		}
		addr = net.IP(tmpAddr[:net.IPv6len]).String()
		port = binary.BigEndian.Uint16(tmpAddr[net.IPv6len:])
	case typeDomain:
		if _, err = io.ReadFull(r, tmp); err != nil {
			return
		}
		domainLen := int(tmp[0])
		tmpAddr := make([]byte, domainLen+2)
		if _, err = io.ReadFull(r, tmpAddr); err != nil {
			return
		}
		addr = string(tmpAddr[:domainLen])
		port = binary.BigEndian.Uint16(tmpAddr[domainLen:])
	default:
		err = fmt.Errorf("address type [ %d ] not supported", addrType&AddrMask)
		return
	}
	addr = net.JoinHostPort(addr, strconv.Itoa(int(port)))
	return
}

// HmacSha1 hmac sha1 with length 10
func HmacSha1(key []byte, data []byte) []byte {
	hmacSha1 := hmac.New(sha1.New, key)
	hmacSha1.Write(data) // nolint: errcheck
	return hmacSha1.Sum(nil)[:10]
}
