package through

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestParseRequest(t *testing.T) {
	tests := []struct {
		name    string
		reader  io.Reader
		wantMsg Request
		wantErr bool
	}{
		{
			"data",
			bytes.NewBuffer([]byte{0x5a, 0xa5, 0x05, 1, 2, 3, 4, 5}),
			Request{
				0x5a & 0x07, 0xa5, []byte{1, 2, 3, 4, 5},
			},
			false,
		},
		{
			"invalid data length",
			bytes.NewBuffer([]byte{0x5a, 0xa5, 0xff, 0xff, 0xff, 1, 2, 3}),
			Request{
				0x5a & 0x07, 0xa5, nil,
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, err := ParseRequest(tt.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMsg, tt.wantMsg) {
				t.Errorf("ParseRequest() gotMsg = %v, want %v", gotMsg, tt.wantMsg)
			}
		})
	}
}

func TestRequest_Bytes(t *testing.T) {
	tests := []struct {
		name    string
		request Request
		want    []byte
		wantErr bool
	}{
		{
			"large than 2097151",
			Request{Data: make([]byte, 2097152)},
			nil,
			true,
		},
		{
			"dataLen = 0",
			Request{0x5a, 0xa5, []byte{}},
			[]byte{0x5a & 0x07, 0xa5, 0x00},
			false,
		},
		{
			"0 <= dataLen =< 127",
			Request{0x5a, 0xa5, make([]byte, 5)},
			append([]byte{0x5a & 0x07, 0xa5, 0x05}, make([]byte, 5)...),
			false,
		},
		{
			"16383 >= dataLen >= 128",
			Request{0x5a, 0xa5, make([]byte, 128)},
			append([]byte{0x5a & 0x07, 0xa5, 0x80, 0x01}, make([]byte, 128)...),
			false,
		},
		{
			"2097151 >= dataLen >= 16384",
			Request{0x5a, 0xa5, make([]byte, 16384)},
			append([]byte{0x5a & 0x07, 0xa5, 0x80, 0x80, 0x01}, make([]byte, 16384)...),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.request.Bytes()
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

func TestRequest_Header(t *testing.T) {
	tests := []struct {
		name    string
		request Request
		want    []byte
		wantErr bool
	}{
		{
			"large than 2097151",
			Request{Data: make([]byte, 2097152)},
			nil,
			true,
		},
		{
			"dataLen = 0",
			Request{0x5a, 0xa5, []byte{}},
			[]byte{0x5a & 0x07, 0xa5, 0x00},
			false,
		},
		{
			"0 <= dataLen =< 127",
			Request{0x5a, 0xa5, make([]byte, 5)},
			[]byte{0x5a & 0x07, 0xa5, 0x05},
			false,
		},
		{
			"16383 >= dataLen >= 128",
			Request{0x5a, 0xa5, make([]byte, 128)},
			[]byte{0x5a & 0x07, 0xa5, 0x80, 0x01},
			false,
		},
		{
			"2097151 >= dataLen >= 16384",
			Request{0x5a, 0xa5, make([]byte, 16384)},
			[]byte{0x5a & 0x07, 0xa5, 0x80, 0x80, 0x01},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.request.Header()
			if (err != nil) != tt.wantErr {
				t.Errorf("Header() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Header() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseReply(t *testing.T) {
	tests := []struct {
		name    string
		r       io.Reader
		wantTr  Reply
		wantErr bool
	}{
		{
			"",
			bytes.NewReader([]byte{RepSuccess, Version}),
			Reply{RepSuccess, Version},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTr, err := ParseReply(tt.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseReply() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotTr, tt.wantTr) {
				t.Errorf("ParseReply() gotTr = %v, want %v", gotTr, tt.wantTr)
			}
		})
	}
}
