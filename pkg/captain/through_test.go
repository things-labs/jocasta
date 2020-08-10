package captain

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestParseMessage(t *testing.T) {
	type args struct {
		r io.Reader
	}
	tests := []struct {
		name    string
		args    args
		wantMsg Through
		wantErr bool
	}{
		{
			"data",
			args{bytes.NewBuffer([]byte{0x5a, 0xa5, 0x05, 1, 2, 3, 4, 5})},
			Through{
				0x5a, 0xa5, []byte{1, 2, 3, 4, 5},
			},
			false,
		},
		{
			"invalid data length",
			args{bytes.NewBuffer([]byte{0x5a, 0xa5, 0xff, 0xff, 0xff, 1, 2, 3})},
			Through{
				0x5a, 0xa5, nil,
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, err := ParseThrough(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseThrough() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMsg, tt.wantMsg) {
				t.Errorf("ParseThrough() gotMsg = %v, want %v", gotMsg, tt.wantMsg)
			}
		})
	}
}

func TestMessage_Bytes(t *testing.T) {
	tests := []struct {
		name    string
		message Through
		want    []byte
		wantErr bool
	}{
		{
			"large than 2097151",
			Through{Data: make([]byte, 2097152)},
			nil,
			true,
		},
		{
			"dataLen = 0",
			Through{0x5a, 0xa5, []byte{}},
			[]byte{0x5a, 0xa5, 0x00},
			false,
		},
		{
			"0 <= dataLen =< 127",
			Through{0x5a, 0xa5, make([]byte, 5)},
			append([]byte{0x5a, 0xa5, 0x05}, make([]byte, 5)...),
			false,
		},
		{
			"16383 >= dataLen >= 128",
			Through{0x5a, 0xa5, make([]byte, 128)},
			append([]byte{0x5a, 0xa5, 0x80, 0x01}, make([]byte, 128)...),
			false,
		},
		{
			"2097151 >= dataLen >= 16384",
			Through{0x5a, 0xa5, make([]byte, 16384)},
			append([]byte{0x5a, 0xa5, 0x80, 0x80, 0x01}, make([]byte, 16384)...),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := Through{
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
