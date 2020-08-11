package captain

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestParseRawThroughRequest(t *testing.T) {
	tests := []struct {
		name    string
		reader  io.Reader
		wantMsg ThroughRequest
		wantErr bool
	}{
		{
			"data",
			bytes.NewBuffer([]byte{0x5a, 0xa5, 0x05, 1, 2, 3, 4, 5}),
			ThroughRequest{
				0x5a & 0x07, 0xa5, []byte{1, 2, 3, 4, 5},
			},
			false,
		},
		{
			"invalid data length",
			bytes.NewBuffer([]byte{0x5a, 0xa5, 0xff, 0xff, 0xff, 1, 2, 3}),
			ThroughRequest{
				0x5a & 0x07, 0xa5, nil,
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, err := ParseRawThroughRequest(tt.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRawThroughRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMsg, tt.wantMsg) {
				t.Errorf("ParseRawThroughRequest() gotMsg = %v, want %v", gotMsg, tt.wantMsg)
			}
		})
	}
}

func TestThroughRequest_Bytes(t *testing.T) {
	tests := []struct {
		name    string
		message ThroughRequest
		want    []byte
		wantErr bool
	}{
		{
			"large than 2097151",
			ThroughRequest{Data: make([]byte, 2097152)},
			nil,
			true,
		},
		{
			"dataLen = 0",
			ThroughRequest{0x5a, 0xa5, []byte{}},
			[]byte{0x5a & 0x07, 0xa5, 0x00},
			false,
		},
		{
			"0 <= dataLen =< 127",
			ThroughRequest{0x5a, 0xa5, make([]byte, 5)},
			append([]byte{0x5a & 0x07, 0xa5, 0x05}, make([]byte, 5)...),
			false,
		},
		{
			"16383 >= dataLen >= 128",
			ThroughRequest{0x5a, 0xa5, make([]byte, 128)},
			append([]byte{0x5a & 0x07, 0xa5, 0x80, 0x01}, make([]byte, 128)...),
			false,
		},
		{
			"2097151 >= dataLen >= 16384",
			ThroughRequest{0x5a, 0xa5, make([]byte, 16384)},
			append([]byte{0x5a & 0x07, 0xa5, 0x80, 0x80, 0x01}, make([]byte, 16384)...),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := ThroughRequest{
				Types:   tt.message.Types,
				Version: tt.message.Version,
				Data:    tt.message.Data,
			}
			got, err := sf.Bytes()
			if (err != nil) != tt.wantErr {
				t.Errorf("Bytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bytes() got = %v, want %v", got, tt.want)
			}
		})
	}
}
