// Package sni implement (Server Name Indication)服务器名称指示,扩展TLS计算机联网协议
// see https://tools.ietf.org/html/rfc6066
package sni

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
)

var malformedError = errors.New("SNI: malformed client hello")

type bufferedConn struct {
	net.Conn
	r io.Reader
}

// Read reads data into p.
func (b *bufferedConn) Read(p []byte) (int, error) {
	return b.r.Read(p)
}

// ServerNameFromBytes get server name from bytes
func ServerNameFromBytes(data []byte) (hostname string, err error) {
	hostname, _, err = ServerNameFromConn(&bufferedConn{nil, bytes.NewReader(data)})
	return
}

// ServerNameFromConn Uses SNI to get the name of the server from the connection.
// Returns the ServerName and a buffered connection that will not have been read off of.
func ServerNameFromConn(c net.Conn) (hostname string, conn net.Conn, err error) {
	var helloBytes []byte

	hostname, helloBytes, err = getServername(c)
	if err != nil {
		return
	}
	conn = &bufferedConn{c, io.MultiReader(bytes.NewReader(helloBytes), c)}
	return
}

func getServername(c net.Conn) (hostname string, all []byte, err error) {
	reader := bufio.NewReader(c)
	var b []byte

	b, err = reader.Peek(5)
	if err != nil {
		return
	}
	// ContentType  handshake
	if b[0] != 0x16 {
		err = errors.New("SNI: not TLS")
		return
	}
	// ProtocolVersion
	if b[1] < 3 || (b[1] == 3 && b[2] < 1) {
		err = errors.New("SNI: expected TLS version >= 3.1")
		return
	}
	// Length max 2^14
	restLength := (int(b[3]) << 8) + int(b[4])
	all, err = reader.Peek(5 + restLength)
	if err != nil {
		return
	}
	// Body
	rest := all[5:]
	if len(rest) == 0 {
		return "", nil, malformedError
	}

	// ClientHello(1)
	handshakeType := rest[0]
	current := 1
	if handshakeType != 0x01 {
		err = errors.New("SNI: not a ClientHello")
		return
	}

	// Skip over message length
	current += 3
	// Skip over ProtocolVersion
	current += 2
	// Skip over GMT Unix timestamp and random number
	current += 4 + 28
	if current > len(rest) {
		err = malformedError
		return
	}

	// Skip over session ID(length + content)
	sessionIDLength := int(rest[current])
	current += 1
	current += sessionIDLength
	if current+1 > len(rest) {
		err = malformedError
		return
	}
	// Skip over CipherSuiteList(length + content)
	cipherSuiteLength := (int(rest[current]) << 8) + int(rest[current+1])
	current += 2
	current += cipherSuiteLength
	if current > len(rest) {
		err = malformedError
		return
	}
	// Skip over CompressionMethod(length + content)
	compressionMethodLength := int(rest[current])
	current += 1
	current += compressionMethodLength
	if current > len(rest) {
		err = errors.New("SNI: no extensions")
		return
	}
	// Skip over extensionsLength
	current += 2

	hostname = ""
	for current+4 < len(rest) && hostname == "" {
		// type
		extensionType := (int(rest[current]) << 8) + int(rest[current+1])
		current += 2
		// length
		extensionDataLength := (int(rest[current]) << 8) + int(rest[current+1])
		current += 2
		// ServerName(0)
		if extensionType == 0 {
			// Skip over number of names as we're assuming there's just one
			current += 2
			if current > len(rest) {
				err = malformedError
				return
			}
			// name type
			nameType := rest[current]
			current += 1
			if nameType != 0 {
				err = errors.New("SNI: extension not a hostname")
				return
			}
			if current+1 > len(rest) {
				err = malformedError
				return
			}
			nameLen := (int(rest[current]) << 8) + int(rest[current+1])
			current += 2
			if current+nameLen > len(rest) {
				err = malformedError
				return
			}
			hostname = string(rest[current : current+nameLen])
		}
		current += extensionDataLength
	}
	if hostname == "" {
		err = errors.New("SNI: no hostname found")
	}
	return
}
