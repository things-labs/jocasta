package captain

import (
	"bytes"
	"io"
	"net"
	"reflect"
	"testing"
)

func TestParseDatagramFromReader(t *testing.T) {
	tests := []struct {
		name    string
		reader  io.Reader
		wantDa  Datagram
		wantErr bool
	}{
		{
			"IPv4",
			bytes.NewReader([]byte{0, ATYPIPv4, 127, 0, 0, 1, 0x1f, 0x90, 0x03, 1, 2, 3}),
			Datagram{
				0, AddrSpec{
					IP:       net.IPv4(127, 0, 0, 1),
					Port:     8080,
					AddrType: ATYPIPv4,
				},
				[]byte{1, 2, 3},
			},
			false,
		},
		{
			"IPv6",
			bytes.NewReader([]byte{0, ATYPIPv6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0x1f, 0x90, 0x03, 1, 2, 3}),
			Datagram{
				0, AddrSpec{
					IP:       net.IPv6loopback,
					Port:     8080,
					AddrType: ATYPIPv6,
				},
				[]byte{1, 2, 3},
			},
			false,
		},
		{
			"FQDN",
			bytes.NewReader([]byte{0, ATYPDomain, 9, 'l', 'o', 'c', 'a', 'l', 'h', 'o', 's', 't', 0x1f, 0x90, 0x03, 1, 2, 3}),
			Datagram{
				0, AddrSpec{
					FQDN:     "localhost",
					Port:     8080,
					AddrType: ATYPDomain,
				},
				[]byte{1, 2, 3},
			},
			false,
		},
		{
			"invalid address type",
			bytes.NewReader([]byte{0, 0x02, 127, 0, 0, 1, 0x1f, 0x90}),
			Datagram{},
			true,
		},
		{
			"less min length",
			bytes.NewReader([]byte{0, ATYPIPv4, 127, 0, 0, 1, 0x1f}),
			Datagram{},
			true,
		},
		{
			"less domain length",
			bytes.NewReader([]byte{0, ATYPDomain, 10, 127, 0, 0, 1, 0x1f, 0x09}),
			Datagram{},
			true,
		},
		{
			"less ipv6 length",
			bytes.NewReader([]byte{0, ATYPIPv6, 127, 0, 0, 1, 0x1f, 0x09}),
			Datagram{},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDa, err := ParseStreamDatagram(tt.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDatagram() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && !reflect.DeepEqual(gotDa, tt.wantDa) {
				t.Errorf("ParseDatagram() gotDa = %v, want %v", gotDa, tt.wantDa)
			}
		})
	}
}
