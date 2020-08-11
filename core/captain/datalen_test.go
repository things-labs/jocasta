package captain

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestDataLen2Bytes(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		want    []byte
		wantN   int
		wantErr bool
	}{
		{
			"large than 2097151",
			2097152,
			[]byte{},
			0,
			true,
		},
		{
			"data < 0",
			-1,
			[]byte{},
			0,
			true,
		},
		{
			"dataLen = 0",
			0,
			[]byte{0x00},
			1,
			false,
		},
		{
			"0 <= dataLen =< 127",
			127,
			[]byte{0x7f},
			1,
			false,
		},
		{
			"16383 >= dataLen >= 128",
			128,
			[]byte{0x80, 0x01},
			2,
			false,
		},
		{
			"2097151 >= dataLen >= 16384",
			16384,
			[]byte{0x80, 0x80, 0x01},
			3,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, n, err := DataLen(tt.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("DataLen() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if n != tt.wantN {
				t.Errorf("DataLen() gotN = %v, wantN %v", n, tt.wantN)
			}
			if !reflect.DeepEqual(got[:n], tt.want) {
				t.Errorf("DataLen() got = %v, want %v", got[:n], tt.want)
			}
		})
	}
}

func TestParseDataLen(t *testing.T) {
	tests := []struct {
		name    string
		reader  io.Reader
		want    int
		wantErr bool
	}{
		{
			"dataLen = 0",
			bytes.NewBuffer([]byte{0}),
			0,
			false,
		},
		{
			"0 <= dataLen =< 127",
			bytes.NewBuffer([]byte{0x05}),
			5,
			false,
		},
		{
			"16383 >= dataLen >= 128",
			bytes.NewBuffer([]byte{0x80, 0x01}),
			128,
			false,
		},
		{
			"2097151 >= dataLen >= 16384",
			bytes.NewBuffer([]byte{0x80, 0x80, 0x01}),
			16384,
			false,
		},
		{
			"invalid data length",
			bytes.NewBuffer([]byte{0xff, 0xff, 0xff}),
			0,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDataLen(tt.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDataLen() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseDataLen() got = %v, want %v", got, tt.want)
			}
		})
	}
}
