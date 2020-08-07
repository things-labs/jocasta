package shadowsocks

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestImproveCoverage(t *testing.T) {
	str := []byte("abc")
	HmacSha1(str, str)
}

func TestParseRequest(t *testing.T) {
	type args struct {
		r io.Reader
	}
	tests := []struct {
		name     string
		args     args
		wantAddr string
		wantErr  bool
	}{
		{
			"shadowsocks IPV4",
			args{bytes.NewReader([]byte{typeIPv4, 127, 0, 0, 1, 0x1f, 0x90})},
			"127.0.0.1:8080",
			false,
		},
		{
			"shadowsocks IPV6",
			args{bytes.NewReader([]byte{typeIPv6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0x1f, 0x90})},
			"[::1]:8080",
			false,
		},
		{
			"shadowsocks FQDN",
			args{bytes.NewReader([]byte{typeDomain, 9, 'l', 'o', 'c', 'a', 'l', 'h', 'o', 's', 't', 0x1f, 0x90})},
			"localhost:8080",
			false,
		},
		{
			"shadowsocks invalid address type",
			args{bytes.NewReader([]byte{2, 0, 0, 0, 0, 0x1f, 0x90})},
			"",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddr, err := ParseRequest(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotAddr != tt.wantAddr {
				t.Errorf("ParseRequest() gotAddr = %v, want %v", gotAddr, tt.wantAddr)
			}
		})
	}
}

func TestParseAddrSpec(t *testing.T) {
	type args struct {
		addr string
	}
	tests := []struct {
		name    string
		args    args
		wantBuf []byte
		wantErr bool
	}{
		{
			"IPv4",
			args{"127.0.0.1:8080"},
			[]byte{typeIPv4, 127, 0, 0, 1, 0x1f, 0x90},
			false,
		},
		{
			"IPv6",
			args{"[::1]:8080"},
			[]byte{typeIPv6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0x1f, 0x90},
			false,
		},
		{
			"FQDN",
			args{"localhost:8080"},
			[]byte{typeDomain, 9, 'l', 'o', 'c', 'a', 'l', 'h', 'o', 's', 't', 0x1f, 0x90},
			false,
		},
		{
			"invalid address,miss port",
			args{"localhost"},
			nil,
			true,
		},
		{
			"invalid port",
			args{"localhost:abc"},
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBuf, err := ParseAddrSpec(tt.args.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAddrSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotBuf, tt.wantBuf) {
				t.Errorf("ParseAddrSpec() gotBuf = %v, want %v", gotBuf, tt.wantBuf)
			}
		})
	}
}
