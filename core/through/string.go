package through

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
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

// 生成一个pack
// 格式: packetType+ (data length(4字节) + data)*n
func BuildStringsWithType(packetType uint8, data ...string) []byte {
	pkg := new(bytes.Buffer)
	_ = binary.Write(pkg, binary.LittleEndian, packetType)
	for _, d := range data {
		bs := []byte(d)
		_ = binary.Write(pkg, binary.LittleEndian, uint64(len(bs)))
		_ = binary.Write(pkg, binary.LittleEndian, bs)
	}
	return pkg.Bytes()
}

//typed packet with string
func ReadStringsWithType(r io.Reader, packetType *uint8, data ...*string) (err error) {
	if err = binary.Read(r, binary.LittleEndian, packetType); err != nil {
		return
	}

	for _, d := range data {
		*d, err = readString(r)
		if err != nil {
			return
		}
	}
	return
}
