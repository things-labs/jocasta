package filter

import (
	"context"
	"net"
	"testing"
	"time"

	cmap "github.com/orcaman/concurrent-map"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thinkgos/jocasta/pkg/logger"
)

func TestItem_isNeedLivenessProde(t *testing.T) {
	tests := []struct {
		name string
		item Item
		want bool
	}{
		{
			"[no need] success > 3, failure < 3, success > failure, lasttime offset < 30Minute",
			Item{
				successCount:   10,
				failureCount:   0,
				lastActiveTime: time.Now().Unix(),
			},
			false,
		},
		{
			"[no need] success > 3, failure > 3, success > failure, lasttime offset < 30Minute",
			Item{
				successCount:   10,
				failureCount:   4,
				lastActiveTime: time.Now().Unix(),
			},
			false,
		},
		{
			"[need] success > 3, failure > 3, success < failure, lasttime offset < 30Minute",
			Item{
				successCount:   4,
				failureCount:   5,
				lastActiveTime: time.Now().Unix(),
			},
			true,
		},
		{
			"[need] success > 3, failure < 3, success > failure, lasttime offset > 30Minute",
			Item{
				successCount:   10,
				failureCount:   0,
				lastActiveTime: time.Now().Add(-time.Minute * 30).Unix(),
			},
			true,
		},
		{
			"[need] success > 3, failure > 3, success > failure, lasttime offset > 30Minute",
			Item{
				successCount:   10,
				failureCount:   4,
				lastActiveTime: time.Now().Add(-time.Minute * 30).Unix(),
			},
			true,
		},
		{
			"[need] success > 3, failure > 3, success < failure, lasttime offset > 30Minute",
			Item{
				successCount:   4,
				failureCount:   5,
				lastActiveTime: time.Now().Add(-time.Minute * 30).Unix(),
			},
			true,
		},
		{
			"[need] success < 3",
			Item{successCount: 2},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.isNeedLivenessProde(defaultThreshold, defaultThreshold, defaultAliveThreshold); got != tt.want {
				t.Errorf("isNeedLivenessProde() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mockGoPool struct{}

func (g mockGoPool) Go(f func()) {
	go f()
}
func (g mockGoPool) Tune(int)     {}
func (g mockGoPool) Running() int { return 0 }
func (g mockGoPool) Free() int    { return 0 }
func (g mockGoPool) Cap() int     { return 0 }

func TestFilter_direct(t *testing.T) {
	filte := New("direct",
		WithLogger(logger.NewDiscard()),
		WithGPool(new(mockGoPool)),
		WithTimeout(time.Second),
		WithLivenessPeriod(time.Second*3),
		WithLivenessProbe(func(ctx context.Context, addr string, timeout time.Duration) error {
			conn, err := net.DialTimeout("tcp", addr, timeout)
			if err != nil {
				return err
			}
			conn.Close() // nolint: errcheck
			return nil
		}),
		WithAliveThreshold(0),
		WithSuccessThreshold(0),
		WithFailureThreshold(0),
	)
	defer filte.Close() // nolint: errcheck

	n, err := filte.LoadProxyFile("./testdata/proxy.txt")
	assert.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, 3, filte.ProxyItemCount())

	n, err = filte.LoadDirectFile("./testdata/direct.txt")
	assert.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, 3, filte.DirectItemCount())

	filte.Add("ca.com", "10.10.10.10")
	filte.Add("cb.com", "10.10.10.11")
	filte.Add("cc.com", "10.10.10.12")

	isProxy, inMap, fail, succ := filte.IsProxy("pa.com")
	assert.True(t, isProxy)
	assert.True(t, inMap)
	assert.Equal(t, uint(0), fail)
	assert.Equal(t, uint(0), succ)

	isProxy, inMap, fail, succ = filte.IsProxy("da.com")
	assert.False(t, isProxy)
	assert.True(t, inMap)
	assert.Equal(t, uint(0), fail)
	assert.Equal(t, uint(0), succ)

	// in cache
	isProxy, inMap, fail, succ = filte.IsProxy("ca.com")
	assert.False(t, isProxy)
	assert.True(t, inMap)
	assert.Equal(t, uint(0), fail)
	assert.Equal(t, uint(0), succ)

	// not in cache
	isProxy, inMap, fail, succ = filte.IsProxy("ia.com")
	assert.True(t, isProxy)
	assert.False(t, inMap)
	assert.Equal(t, uint(0), fail)
	assert.Equal(t, uint(0), succ)
}

func TestFilter_Proxy(t *testing.T) {
	filte := New("proxy")
	defer filte.Close() // nolint: errcheck

	filte.Add("ca.com", "10.10.10.10")
	filte.Add("cb.com", "10.10.10.11")
	filte.Add("cc.com", "10.10.10.12")

	isProxy, inMap, fail, succ := filte.IsProxy("ca.com")
	assert.True(t, isProxy)
	assert.True(t, inMap)
	assert.Equal(t, uint(0), fail)
	assert.Equal(t, uint(0), succ)
}

func TestFilter_intelligent(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				return
			}
		}
	}()

	filte := New("intelligent",
		WithLivenessPeriod(time.Second),
	)
	defer filte.Close() // nolint: errcheck

	filte.Add("localhost", ln.Addr().String())

	isProxy, inMap, fail, succ := filte.IsProxy("localhost")
	assert.False(t, isProxy)
	assert.True(t, inMap)
	assert.Equal(t, uint(0), fail)
	assert.Equal(t, uint(0), succ)
	time.Sleep(time.Second * 5)
}

func TestFilter_Match(t *testing.T) {
	directs := cmap.New()
	directs.Set("bar.com", struct{}{})

	proxies := cmap.New()
	proxies.Set("foo.com", struct{}{})

	filte := &Filter{directs: directs, proxies: proxies}

	type args struct {
		domain  string
		isProxy bool
	}
	tests := []struct {
		name  string
		filte *Filter
		args  args
		want  bool
	}{
		{
			"in direct",
			filte,
			args{"bar.com", false},
			true,
		},

		{
			"not direct",
			filte,
			args{"baa.com", false},
			false,
		},
		{
			"in proxy",
			filte,
			args{"foo.com", true},
			true,
		},
		{
			"not direct",
			filte,
			args{"faa.com", true},
			false,
		},
		{
			"invalid domain",
			filte,
			args{"", false},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filte.Match(tt.args.domain, tt.args.isProxy); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_loadfile2ConcurrentMap(t *testing.T) {
	cmp := cmap.New()

	n, err := loadfile2ConcurrentMap(&cmp, "notExist.txt")
	require.NoError(t, err)
	require.Equal(t, 0, n)

	n, err = loadfile2ConcurrentMap(&cmp, "./testdata/direct.txt")
	require.NoError(t, err)
	require.Equal(t, 3, n)
}

func Test_hostname(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		want   string
	}{
		{
			"only host",
			"localhost",
			"localhost",
		},
		{
			"host:port",
			"localhost:8080",
			"localhost",
		},
		{
			"http://host:port",
			"http://localhost:8080",
			"localhost",
		},
		{
			"https://host:port",
			"https://localhost:8080",
			"localhost",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hostname(tt.domain); got != tt.want {
				t.Errorf("hostname() = %v, want %v", got, tt.want)
			}
		})
	}
}
