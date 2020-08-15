package loadbalance

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpstream(t *testing.T) {
	_, err := NewUpstream(Config{})
	require.Error(t, err)

	// start a server
	ln, err := net.Listen("tcp", ":")
	require.NoError(t, err)
	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				return
			}
		}
	}()
	defer ln.Close() // nolint: errcheck

	ups, err := NewUpstream(Config{
		Addr:             ln.Addr().String(),
		Weight:           0,
		MaxConnections:   5,
		SuccessThreshold: 0,
		FailureThreshold: 0,
		Period:           0,
		Timeout:          0,
		LivenessProbe:    tcpLivenessProbe,
	})
	require.NoError(t, err)

	assert.Equal(t, time.Duration(0), ups.LeastTime())
	assert.Equal(t, int64(0), ups.ConnsCount())
	ups.ConnsIncrease()
	ups.ConnsIncrease()
	assert.Equal(t, int64(2), ups.ConnsCount())
	ups.ConnsDecrease()
	assert.Equal(t, int64(1), ups.ConnsCount())
	ups.ConnsDecrease()
	assert.Equal(t, int64(0), ups.ConnsCount())

	assert.False(t, ups.Healthy())
	assert.False(t, ups.Available())
	assert.False(t, ups.Full())

	// success healthy check
	for i := 0; i < 4; i++ {
		ups.ConnsIncrease() // just for testing
		ups.healthyCheck(ln.Addr().String())
	}

	// connection not full
	assert.True(t, ups.Healthy())
	assert.True(t, ups.Available())
	assert.False(t, ups.Full())

	// connection full
	ups.ConnsIncrease()
	assert.False(t, ups.Available())
	assert.True(t, ups.Full())

	ups.ConnsDecrease()

	// failure healthy check but not reach threshold
	for i := 0; i < 2; i++ {
		ups.ConnsDecrease() // just for testing
		ups.healthyCheck("invalid")
	}

	assert.True(t, ups.Healthy())
	assert.True(t, ups.Available())
	assert.False(t, ups.Full())

	// failure healthy check reach threshold
	for i := 0; i < 2; i++ {
		ups.ConnsDecrease() // just for testing
		ups.healthyCheck("invalid")
	}

	assert.False(t, ups.Healthy())
	assert.False(t, ups.Available())
	assert.False(t, ups.Full())
}

func TestUpstreamPool(t *testing.T) {
	// pool
	p1, _ := NewUpstream(Config{Addr: ":81"})
	p2, _ := NewUpstream(Config{Addr: ":82"})
	p3, _ := NewUpstream(Config{Addr: ":83"})
	pool := UpstreamPool{p1, p2, p3}

	pool.ConnsIncrease("invalid")
	pool.ConnsIncrease(":81")
	pool.ConnsIncrease(":83")
	pool.ConnsIncrease(":83")
	require.Equal(t, int64(1), p1.ConnsCount())
	require.Equal(t, int64(0), p2.ConnsCount())
	require.Equal(t, int64(2), p3.ConnsCount())

	pool.ConnsDecrease(":81")
	pool.ConnsDecrease(":83")
	pool.ConnsDecrease(":83")
	require.Equal(t, int64(0), p1.ConnsCount())
	require.Equal(t, int64(0), p2.ConnsCount())
	require.Equal(t, int64(0), p3.ConnsCount())

	assert.False(t, pool.HasHealthy())
	assert.Equal(t, 0, pool.HealthyCount())

	atomic.StoreUint32(&p1.health, 1)

	assert.True(t, pool.HasHealthy())
	assert.Equal(t, 1, pool.HealthyCount())

	// improve code coverage
	pool = NewUpstreamPool([]Config{Config{Addr: ":81"}, Config{Addr: ":81"}, Config{Addr: "invalid"}})

	assert.Equal(t, 2, len(pool))
}
