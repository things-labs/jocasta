package shadowsocks

import (
	"bytes"
	"io"
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
