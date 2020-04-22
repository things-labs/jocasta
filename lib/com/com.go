package com

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/rs/xid"
)

func SubStr(str string, start, length int) string {
	if len(str) == 0 {
		return ""
	}
	rs := []rune(str)
	end := start + length - 1

	if start < 0 {
		start = 0
	}
	if end < 0 || end >= len(str) {
		end = len(rs) - 1
	}
	return string(rs[start:end])
}

func SubBytes(b []byte, start, length int) []byte {
	if len(b) == 0 {
		return []byte{}
	}

	end := start + length - 1

	if end < 0 || end >= len(b) {
		end = len(b) - 1
	}
	return b[start:end]
}

func UniqueId() string {
	str := fmt.Sprintf("%d%s", time.Now().UnixNano(), xid.New().String())
	hash := sha1.New()
	hash.Write([]byte(str))
	return hex.EncodeToString(hash.Sum(nil))
}
