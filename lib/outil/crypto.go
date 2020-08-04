package outil

import (
	"crypto/md5"
	"encoding/hex"

	"github.com/thinkgos/jocasta/internal/bytesconv"
)

// MD5Hex md5 string to hex string
func MD5Hex(str string) string {
	hash := md5.New()
	hash.Write(bytesconv.Str2Bytes(str)) // nolint: errcheck
	return hex.EncodeToString(hash.Sum(nil))
}
