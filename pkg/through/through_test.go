package through

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
		wantMsg Message
		wantErr bool
	}{
		{
			"data",
			args{bytes.NewBuffer([]byte{0x5a, 0xa5, 0x05, 1, 2, 3, 4, 5})},
			Message{
				0x5a, 0xa5, []byte{1, 2, 3, 4, 5},
			},
			false,
		},
		{
			"invalid data length",
			args{bytes.NewBuffer([]byte{0x5a, 0xa5, 0xff, 0xff, 0xff, 1, 2, 3})},
			Message{
				0x5a, 0xa5, nil,
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, err := ParseMessage(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMsg, tt.wantMsg) {
				t.Errorf("ParseMessage() gotMsg = %v, want %v", gotMsg, tt.wantMsg)
			}
		})
	}
}

func TestDataLen2Bytes(t *testing.T) {
	type args struct {
		length int
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			"large than 2097151",
			args{2097152},
			nil,
			true,
		},
		{
			"data < 0",
			args{-1},
			nil,
			true,
		},
		{
			"dataLen = 0",
			args{0},
			[]byte{0x00},
			false,
		},
		{
			"0 <= dataLen =< 127",
			args{127},
			[]byte{0x7f},
			false,
		},
		{
			"16383 >= dataLen >= 128",
			args{128},
			[]byte{0x80, 0x01},
			false,
		},
		{
			"2097151 >= dataLen >= 16384",
			args{16384},
			[]byte{0x80, 0x80, 0x01},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DataLen2Bytes(tt.args.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("DataLen2Bytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DataLen2Bytes() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_Bytes(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		want    []byte
		wantErr bool
	}{
		{
			"large than 2097151",
			Message{Data: make([]byte, 2097152)},
			nil,
			true,
		},
		{
			"dataLen = 0",
			Message{0x5a, 0xa5, []byte{}},
			[]byte{0x5a, 0xa5, 0x00},
			false,
		},
		{
			"0 <= dataLen =< 127",
			Message{0x5a, 0xa5, make([]byte, 5)},
			append([]byte{0x5a, 0xa5, 0x05}, make([]byte, 5)...),
			false,
		},
		{
			"16383 >= dataLen >= 128",
			Message{0x5a, 0xa5, make([]byte, 128)},
			append([]byte{0x5a, 0xa5, 0x80, 0x01}, make([]byte, 128)...),
			false,
		},
		{
			"2097151 >= dataLen >= 16384",
			Message{0x5a, 0xa5, make([]byte, 16384)},
			append([]byte{0x5a, 0xa5, 0x80, 0x80, 0x01}, make([]byte, 16384)...),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := Message{
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
