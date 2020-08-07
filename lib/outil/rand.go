package outil

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/rs/xid"

	"github.com/thinkgos/jocasta/internal/bytesconv"
)

const codesString = "QWERTYUIOPLKJHGFDSAZXCVBNMabcdefghijklmnopqrstuvwxyz0123456789"
const codesInt = "123456789"

// RandString rand string  with give length
func RandString(length int) string {

	r := rand.New(rand.NewSource(time.Now().UnixNano() + rand.Int63() +
		rand.Int63() + rand.Int63() + rand.Int63()))
	result := make([]byte, 0, length)
	for i := 0; i < length; i++ {
		result = append(result, codesString[r.Intn(len(codesString))])
	}
	return bytesconv.Bytes2Str(result)
}

// RandInt rand int with give length
func RandInt(length int) int64 {
	r := rand.New(rand.NewSource(time.Now().UnixNano() + rand.Int63() +
		rand.Int63() + rand.Int63() + rand.Int63()))
	result := make([]byte, 0, length)
	for i := 0; i < length; i++ {
		result = append(result, codesInt[r.Intn(len(codesInt))])
	}
	val, _ := strconv.ParseInt(bytesconv.Bytes2Str(result), 10, 64)
	return val
}

// UniqueID unique id
func UniqueID() string {
	str := fmt.Sprintf("%d%s", time.Now().UnixNano(), xid.New().String())
	hash := sha1.New()
	hash.Write(bytesconv.Str2Bytes(str)) // nolint: errcheck
	return hex.EncodeToString(hash.Sum(nil))
}
