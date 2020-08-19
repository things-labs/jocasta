package through

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/thinkgos/jocasta/lib/extnet"
)

// 格式: data length(4字节) + data
func readByte(r io.Reader) ([]byte, error) {
	var length uint64

	err := binary.Read(r, binary.LittleEndian, &length)
	if err != nil {
		return nil, err
	}
	if length == 0 || length > ^uint64(0) {
		return nil, fmt.Errorf("data len out of range, %d", length)
	}

	data := make([]byte, length)
	if _, err = io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

func readString(r io.Reader) (string, error) {
	data, err := readByte(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// 格式 data length(4字节) + data
func BuildString(data ...string) []byte {
	buf := new(bytes.Buffer)
	for _, d := range data {
		bs := []byte(d)
		_ = binary.Write(buf, binary.LittleEndian, uint64(len(bs)))
		_ = binary.Write(buf, binary.LittleEndian, bs)
	}
	return buf.Bytes()
}

// non typed packet with string
func ReadString(r io.Reader, data ...*string) (err error) {
	for _, d := range data {
		*d, err = readString(r)
		if err != nil {
			return
		}
	}
	return
}

func WriteStrings(conn net.Conn, timeout time.Duration, data ...string) error {
	return extnet.WrapWriteTimeout(conn, timeout, func(c net.Conn) error {
		_, err := c.Write(BuildString(data...))
		return err
	})
}

func ReadStrings(conn net.Conn, timeout time.Duration, data ...*string) error {
	return extnet.WrapReadTimeout(conn, timeout, func(c net.Conn) error {
		return ReadString(conn, data...)
	})
}
