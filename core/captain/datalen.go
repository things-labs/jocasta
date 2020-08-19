package captain

import (
	"errors"
	"io"
)

// Data len
// 第8位表示后续还有值,最长长度字节只能到三字节. 理论上Data可达2097121(2M)
// +-Digits-+----------FROM------------+-----------TO---------------+
// |    1   | 0 (0x00)                 | 127 (0x7f)                 |
// +--------+--------------------------+----------------------------+
// |    2   | 128 (0x80, 0x01)         | 16383 (0x80, 0x7F)         |
// +--------+--------------------------+----------------------------+
// |    3   | 16384 (0x80, 0x80, 0x01) | 2097151 (0xFF, 0xFF, 0x7F) |
// +---------+-------------------------+----------------------------+

// DataLen convert data length to bytes return array and bytes count
func DataLen(length int) (ds [3]byte, n int, err error) {
	if length < 0 || length > 2097151 {
		return ds, n, errors.New("invalid data length")
	}

	for {
		b := byte(length & 0x7f)
		if length = length >> 7; length > 0 {
			b |= 0x80
		}
		ds[n] = b
		if n++; !((n < 3) && length > 0) {
			break
		}
	}
	return
}

// ParseDataLen parse data length from reader
func ParseDataLen(r io.Reader) (int, error) {
	tmp := []byte{0}
	// read remain data len
	length, remain := 0, byte(0)
	for i := 0; ; i++ {
		if _, err := io.ReadFull(r, tmp); err != nil {
			return 0, err
		}
		length += int(tmp[0]&0x7f) << (7 * i)
		if remain = tmp[0] & 0x80; !((remain == 0x80) && (i < 2)) {
			break
		}
	}
	if remain == 0x80 { // max data length should be less than 3
		return 0, errors.New("invalid data length")
	}
	return length, nil
}
