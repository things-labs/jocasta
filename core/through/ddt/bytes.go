// 自定义传输协议
package ddt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// 格式: data length(4字节) + data
func ReadByte(r io.Reader) ([]byte, error) {
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

// 格式: data length(4字节) + data
func BuildBytes(data ...[]byte) []byte {
	pkg := new(bytes.Buffer)
	for _, d := range data {
		_ = binary.Write(pkg, binary.LittleEndian, uint64(len(d)))
		_ = binary.Write(pkg, binary.LittleEndian, d)
	}
	return pkg.Bytes()
}

//non typed packet with Bytes
func ReadBytes(r io.Reader, data ...*[]byte) (err error) {
	for _, d := range data {
		*d, err = ReadByte(r)
		if err != nil {
			return
		}
	}
	return
}

func BuildBytesWithType(packetType uint8, data ...[]byte) []byte {
	pkg := new(bytes.Buffer)
	_ = binary.Write(pkg, binary.LittleEndian, packetType)
	for _, d := range data {
		_ = binary.Write(pkg, binary.LittleEndian, uint64(len(d)))
		_ = binary.Write(pkg, binary.LittleEndian, d)
	}
	return pkg.Bytes()
}

//typed packet with bytes
func ReadBytesWithType(r io.Reader, packetType *uint8, data ...*[]byte) (err error) {
	if err = binary.Read(r, binary.LittleEndian, packetType); err != nil {
		return
	}

	for _, d := range data {
		*d, err = ReadByte(r)
		if err != nil {
			return
		}
	}
	return
}
