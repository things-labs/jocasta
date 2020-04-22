package com

import (
	"math/rand"
	"strconv"
	"time"
)

func RandString(length int) string {
	const codes = "QWERTYUIOPLKJHGFDSAZXCVBNMabcdefghijklmnopqrstuvwxyz0123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano() + rand.Int63() +
		rand.Int63() + rand.Int63() + rand.Int63()))
	result := make([]byte, 0, length)
	for i := 0; i < length; i++ {
		result = append(result, codes[r.Intn(len(codes))])
	}
	return string(result)
}

func RandInt(length int) (val int64) {
	const codes = "123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano() + rand.Int63() +
		rand.Int63() + rand.Int63() + rand.Int63()))
	result := make([]byte, 0, length)
	for i := 0; i < length; i++ {
		result = append(result, codes[r.Intn(len(codes))])
	}
	val, _ = strconv.ParseInt(string(result), 10, 64)
	return
}
