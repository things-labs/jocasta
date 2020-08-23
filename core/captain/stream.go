package captain

import (
	"io"
)

// Request protocol request
// handshake request/response is formed as follows:
// +---------+-------+------------+----------+
// |  TYPES  |  VER  |  DATA_LEN  |   DATA   |
// +---------+-------+------------+----------+
// |    1    |   1   |    1 - 3   | Variable |
// +---------+-------+------------+----------+
// TTYPES 类型
// VER 版本
// DATA_LEN see package data length defined
// DATA 数据
type Request struct {
	Types   byte
	Version byte
	Data    []byte
}

// ParseRequest parse to Request
func ParseRequest(r io.Reader) (req Request, err error) {
	// read request type and version
	tmp := []byte{0, 0}
	if _, err = io.ReadFull(r, tmp); err != nil {
		return
	}
	req.Types, req.Version = tmp[0], tmp[1]

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
	req.Data = data
	return
}

// Bytes request to bytes
func (sf Request) Bytes() ([]byte, error) {
	return sf.value(true)
}

// Header returns s slice of datagram header except data
func (sf Request) Header() ([]byte, error) {
	return sf.value(false)
}

func (sf Request) value(hasData bool) (bs []byte, err error) {
	ds, n, err := DataLen(len(sf.Data))
	if err != nil {
		return nil, err
	}
	if hasData {
		bs = make([]byte, 0, 2+n+len(sf.Data))
	} else {
		bs = make([]byte, 0, 2+n)
	}
	bs = append(bs, sf.Types, sf.Version)
	bs = append(bs, ds[:n]...)
	if hasData {
		bs = append(bs, sf.Data...)
	}
	return bs, nil
}

// Reply protocol reply
// handshake response is formed as follows:
// +---------+-------+
// |  STATUS |  VER  |
// +---------+-------+
// |    1    |   1   |
// +---------+-------+
// STATUS 状态
// VER 版本
type Reply struct {
	Status  byte
	Version byte
}

// ParseReply parse to Reply
func ParseReply(r io.Reader) (reply Reply, err error) {
	// read status and version
	tmp := []byte{0, 0}
	if _, err = io.ReadFull(r, tmp); err != nil {
		return
	}
	reply.Status, reply.Version = tmp[0], tmp[1]
	return
}

// SendReply send reply
func SendReply(conn io.Writer, status byte, version byte) (err error) {
	_, err = conn.Write([]byte{status, version})
	return
}
