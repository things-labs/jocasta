package through

import (
	"errors"
	"io"
)

// Message through protocol message
// handshake request/response is formed as follows:
// +---------+-------+------------+----------+
// |  MTYPE  |  VER  |  DATA_LEN  |   DATA   |
// +---------+-------+------------+----------+
// |    1    |   1   |    1 - 3   | Variable |
// +---------+-------+------------+----------+
//
// Data len
// 第8位表示还有值,最长长度字节只能到三字节. 理论上Data可达2097121(2M)
// +-Digits-+----------FROM------------+-----------TO---------------+
// |    1   | 0 (0x00)                 | 127 (0x7f)                 |
// +--------+--------------------------+----------------------------+
// |    2   | 128 (0x80, 0x01)         | 16383 (0x80, 0x7F)         |
// +--------+--------------------------+----------------------------+
// |    3   | 16384 (0x80, 0x80, 0x01) | 2097151 (0xFF, 0xFF, 0x7F) |
// +---------+-------------------------+----------------------------+
type Message struct {
	Types   byte
	Version byte
	Data    []byte
}

// ParseMessage parse to message
func ParseMessage(r io.Reader) (msg Message, err error) {
	tmp := []byte{0, 0, 0}

	// read message type,version,first data length byte
	if _, err = io.ReadFull(r, tmp); err != nil {
		return
	}
	msg.Types, msg.Version = tmp[0], tmp[1]

	// read remain data len
	length := int((tmp[2] & 0x7f))
	remain := tmp[2] & 0x80
	for i := 0; (remain == 0x80) && (i < 2); i++ {
		if _, err = io.ReadFull(r, tmp[:1]); err != nil {
			return
		}
		length += int(tmp[0]&0x7f) << (7 * (i + 1))
		remain = tmp[0] & 0x80
	}
	if remain == 0x80 { // max data length should be less than 3
		err = errors.New("invalid data length")
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

func DataLen2Bytes(length int) ([]byte, error) {
	if length < 0 || length > 2097151 {
		return nil, errors.New("invalid data length")
	}
	bs := make([]byte, 0, 3)
	for i := 0; ; {
		b := byte(length & 0x7f)
		if length = length >> 7; length > 0 {
			b |= 0x80
		}
		bs = append(bs, b)
		if i++; !((i < 3) && length > 0) {
			break
		}
	}
	return bs, nil
}

func (sf Message) Bytes() ([]byte, error) {
	if len(sf.Data) > 2097151 {
		return nil, errors.New("invalid data length")
	}

	bs := make([]byte, 0, 5+len(sf.Data))
	bs = append(bs, sf.Types, sf.Version)

	length := len(sf.Data)
	for i := 0; ; {
		b := byte(length & 0x7f)
		if length = length >> 7; length > 0 {
			b |= 0x80
		}
		bs = append(bs, b)
		if i++; !((i < 3) && length > 0) {
			break
		}
	}
	bs = append(bs, sf.Data...)
	return bs, nil
}
