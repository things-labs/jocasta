package extnet

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testError struct {
	timeout bool
	err     string
}

func (e *testError) Error() string   { return e.err }
func (e *testError) Timeout() bool   { return e.timeout }
func (e *testError) Temporary() bool { return true }

func TestErr(t *testing.T) {
	assert.True(t, IsErrClosed(errors.New("use of closed network connection")))
	assert.False(t, IsErrClosed(nil))

	assert.True(t, IsErrTimeout(&testError{true, "timeout"}))
	assert.False(t, IsErrTimeout(nil))

	assert.True(t, IsErrRefused(errors.New("connection refused")))
	assert.False(t, IsErrRefused(nil))

	assert.True(t, IsErrDeadline(errors.New("i/o deadline reached")))
	assert.False(t, IsErrDeadline(nil))

	assert.True(t, IsErrSocketNotConnected(errors.New("socket is not connected")))
	assert.False(t, IsErrSocketNotConnected(nil))

}

func TestIsDomain(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		{
			"domain",
			"localhost",
			true,
		},
		{
			"ip",
			"127.0.0.1",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDomain(tt.host); got != tt.want {
				t.Errorf("IsDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsIntranet(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		{
			"ipv4 Loopback 127.0.0.0~127.255.255.255",
			"127.1.1.1",
			true,
		},
		{
			"ipv4 Loopback localhost",
			"localhost",
			true,
		},
		{
			"ipv6 Loopback",
			net.IPv6loopback.String(),
			true,
		},

		{
			"A类10.0.0.0~10.255.255.255",
			"10.1.1.1",
			true,
		},
		{
			"not in A类10.0.0.0~10.255.255.255",
			"11.1.1.1",
			false,
		},
		{
			"b类172.16.0.0~172.31.255.255",
			"172.16.1.1",
			true,
		},
		{
			"1 - not in b类172.16.0.0~172.31.255.255 ",
			"172.15.1.1",
			false,
		},
		{
			"2 - not in b类172.16.0.0~172.31.255.255",
			"172.32.1.1",
			false,
		},
		{
			"c类192.168.0.0~192.168.255.255",
			"192.168.1.1",
			true,
		},
		{
			"not in c类192.168.0.0~192.168.255.255",
			"192.169.1.1",
			false,
		},
		{
			"not intranet",
			"www.baidu.com",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsIntranet(tt.host); got != tt.want {
				t.Errorf("IsIntranet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsHTTP(t *testing.T) {
	assert.True(t, IsHTTP([]byte("get")))
	assert.True(t, IsHTTP([]byte("GET")))

	assert.False(t, IsHTTP([]byte("Get")))
	assert.False(t, IsHTTP([]byte("false")))
}

func BenchmarkIsIntranet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IsIntranet("192.168.1.1")
	}
}

func BenchmarkIsHTTP(b *testing.B) {
	v := []byte("abcedefad")
	for i := 0; i < b.N; i++ {
		IsHTTP(v)
	}
}
