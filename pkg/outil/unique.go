package outil

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/rs/xid"

	"github.com/thinkgos/jocasta/internal/bytesconv"
)

// UniqueID unique id
func UniqueID() string {
	str := fmt.Sprintf("%d%s", time.Now().UnixNano(), xid.New().String())
	hash := sha1.New()
	hash.Write(bytesconv.Str2Bytes(str)) // nolint: errcheck
	return hex.EncodeToString(hash.Sum(nil))
}
