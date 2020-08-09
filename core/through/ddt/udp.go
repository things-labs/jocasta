package ddt

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
)

// 穿透使用,UDP包格式 addr length(2字节) + addr + data length(2字节) + data

// 生成一个udp包
func BuildUDPPacket(addr string, data []byte) []byte {
	addrBs := []byte(addr)
	pkg := new(bytes.Buffer)
	_ = binary.Write(pkg, binary.LittleEndian, uint16(len(addrBs)))
	_ = binary.Write(pkg, binary.LittleEndian, addrBs)
	_ = binary.Write(pkg, binary.LittleEndian, uint16(len(data)))
	_ = binary.Write(pkg, binary.LittleEndian, data)
	return pkg.Bytes()
}

// 解析udp包
func ReadUDPPacket(r io.Reader) (string, []byte, error) {
	var addrLen uint16
	var dataLen uint16

	reader := bufio.NewReader(r)
	// addr length
	err := binary.Read(reader, binary.LittleEndian, &addrLen)
	if err != nil {
		return "", nil, err
	}
	// addr
	addr := make([]byte, addrLen)
	if _, err = io.ReadFull(reader, addr); err != nil {
		return "", nil, err
	}
	// data length
	if err = binary.Read(reader, binary.LittleEndian, &dataLen); err != nil {
		return "", nil, err
	}
	// data
	data := make([]byte, dataLen)
	if _, err = io.ReadFull(reader, data); err != nil {
		return "", nil, err
	}
	return string(addr), data, nil
}
