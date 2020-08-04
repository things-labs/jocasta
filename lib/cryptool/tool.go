package cryptool

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"

	"github.com/thinkgos/jocasta/internal/bytesconv"
)

func Base64Encode(b []byte) []byte {
	buf := make([]byte, base64.StdEncoding.EncodedLen(len(b)))
	base64.StdEncoding.Encode(buf, b)
	return buf
}
func Base64EncodeString(str string) string {
	return base64.StdEncoding.EncodeToString(bytesconv.Str2Bytes(str))
}
func Base64Decode(str string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(str)
}

func Base64DecodeString(str string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(str)
	return string(b), err
}

func MD5Hex(str string) string {
	hash := md5.New()
	_, _ = hash.Write([]byte(str))
	return hex.EncodeToString(hash.Sum(nil))
}
