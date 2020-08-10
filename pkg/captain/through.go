// Package captain 定义了透传,各底层协议转换(数据报->数据流,数据流->数据报的转换)
package captain

import (
	"io"
)

// Message through protocol message
// handshake request/response is formed as follows:
// +---------+-------+------------+----------+
// |  MTYPE  |  VER  |  DATA_LEN  |   DATA   |
// +---------+-------+------------+----------+
// |    1    |   1   |    1 - 3   | Variable |
// +---------+-------+------------+----------+
// DATA_LEN see data length defined
type Message struct {
	Types   byte
	Version byte
	Data    []byte
}

// ParseMessage parse to message
func ParseMessage(r io.Reader) (msg Message, err error) {
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

func (sf Message) Bytes() ([]byte, error) {
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
