package outil

import (
	"encoding/base64"

	"github.com/thinkgos/jocasta/internal/bytesconv"
)

// Base64Encode base64 encode bytes
func Base64Encode(b []byte) []byte {
	buf := make([]byte, base64.StdEncoding.EncodedLen(len(b)))
	base64.StdEncoding.Encode(buf, b)
	return buf
}

// Base64EncodeString base64 encode string
func Base64EncodeString(str string) string {
	return base64.StdEncoding.EncodeToString(bytesconv.Str2Bytes(str))
}

// Base64Decode base64 decode to bytes
func Base64Decode(str string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(str)
}

// Base64DecodeString base64 decode to string
func Base64DecodeString(str string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(str)
	return bytesconv.Bytes2Str(b), err
}
