package captain

import (
	"bytes"
	"io"
	"net"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStreamDatagram(t *testing.T) {
	t.Run("invalid address", func(t *testing.T) {
		_, err := NewStreamDatagram("localhost", nil)
		require.Error(t, err)
	})
	t.Run("domain host name to long", func(t *testing.T) {
		_, err := NewStreamDatagram("localhostlocalhostlocalhostlocalhostlocalhostlocalhostlocalhostlocalhostlocalhostlocalhost"+
			"localhostlocalhostlocalhostlocalhostlocalhostlocalhostlocalhostlocalhostlocalhostlocalhost"+
			"localhostlocalhostlocalhostlocalhostlocalhostlocalhostlocalhostlocalhostlocalhostlocalhost:8080", nil)
		require.Error(t, err)
	})
	t.Run("domain", func(t *testing.T) {
		datagram, err := NewStreamDatagram("localhost:8080", []byte{1, 2, 3})
		require.NoError(t, err)
		require.Equal(t, StreamDatagram{
			0, AddrSpec{
				FQDN:     "localhost",
				Port:     8080,
				AddrType: ATYPDomain,
			},
			[]byte{1, 2, 3},
		}, datagram)
		hd, err := datagram.Header()
		require.NoError(t, err)
		require.Equal(t, []byte{0, ATYPDomain, 9, 'l', 'o', 'c', 'a', 'l', 'h', 'o', 's', 't', 0x1f, 0x90, 0x03}, hd)
		val, err := datagram.Bytes()
		require.NoError(t, err)
		require.Equal(t, []byte{0, ATYPDomain, 9, 'l', 'o', 'c', 'a', 'l', 'h', 'o', 's', 't', 0x1f, 0x90, 0x03, 1, 2, 3}, val)
	})
	t.Run("ipv4", func(t *testing.T) {
		datagram, err := NewStreamDatagram("127.0.0.1:8080", []byte{1, 2, 3})
		require.NoError(t, err)
		require.Equal(t, StreamDatagram{
			0, AddrSpec{
				IP:       net.IPv4(127, 0, 0, 1),
				Port:     8080,
				AddrType: ATYPIPv4,
			},
			[]byte{1, 2, 3},
		}, datagram)
		hd, err := datagram.Header()
		require.NoError(t, err)
		require.Equal(t, []byte{0, ATYPIPv4, 127, 0, 0, 1, 0x1f, 0x90, 0x03}, hd)
		val, err := datagram.Bytes()
		require.NoError(t, err)
		require.Equal(t, []byte{0, ATYPIPv4, 127, 0, 0, 1, 0x1f, 0x90, 0x03, 1, 2, 3}, val)
	})
	t.Run("ipv6", func(t *testing.T) {
		datagram, err := NewStreamDatagram("[::1]:8080", []byte{1, 2, 3})
		require.NoError(t, err)
		require.Equal(t, StreamDatagram{
			0, AddrSpec{
				IP:       net.IPv6loopback,
				Port:     8080,
				AddrType: ATYPIPv6,
			},
			[]byte{1, 2, 3},
		}, datagram)
		hd, err := datagram.Header()
		require.NoError(t, err)
		require.Equal(t, []byte{0, ATYPIPv6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0x1f, 0x90, 0x03}, hd)
		val, err := datagram.Bytes()
		require.NoError(t, err)
		require.Equal(t, []byte{0, ATYPIPv6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0x1f, 0x90, 0x03, 1, 2, 3}, val)
	})
}

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
			"invalid length",
			bytes.NewReader([]byte{0, ATYPIPv4, 127, 0, 0, 1, 0xff, 0xff, 0xff}),
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
