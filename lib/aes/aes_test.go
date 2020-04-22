package goaes

import (
	"bytes"
	"testing"
)

func TestCFB(t *testing.T) {
	key := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	want := []byte("helloworld")
	text, err := EncryptCFB(key, want)
	if err != nil {
		t.Fatalf("EncryptCFB %+v", err)
	}
	got, err := DecryptCFB(key, text)
	if err != nil {
		t.Fatalf("DecryptCFB %+v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got(%s) not want(%s)", got, want)
	}
}

func TestCBC(t *testing.T) {
	key := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	want := []byte("helloworld")
	text, err := EncryptCBC(key, want)
	if err != nil {
		t.Fatalf("EncryptCFB %+v", err)
	}
	got, err := DecryptCBC(key, text)
	if err != nil {
		t.Fatalf("DecryptCFB %+v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got(%s) not want(%s)", got, want)
	}
}
