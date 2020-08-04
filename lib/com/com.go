package com

// SubStr sub string
func SubStr(str string, start, length int) string {
	if len(str) == 0 {
		return ""
	}
	rs := []rune(str)

	if start < 0 {
		start = 0
	}
	end := start + length
	if end < 0 || end >= len(str) {
		end = len(rs)
	}
	return string(rs[start:end])
}

// SubBytes sub bytes
func SubBytes(b []byte, start, length int) []byte {
	if len(b) == 0 {
		return b
	}
	if start < 0 {
		start = 0
	}
	end := start + length
	if end < 0 || end >= len(b) {
		end = len(b)
	}
	return b[start:end]
}
