package loadbalance

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thinkgos/go-core-package/lib/logger"
	"github.com/thinkgos/jocasta/core/idns"
)

type testSelector struct{}

func (testSelector) Select(UpstreamPool, string) *Upstream {
	return nil
}

func TestSelector(t *testing.T) {
	t.Run("RegisterSelector", func(t *testing.T) {
		assert.Panics(t, func() { RegisterSelector("", nil) })
		assert.Panics(t, func() { RegisterSelector("", func() Selector { return nil }) })
		assert.Panics(t, func() { RegisterSelector("roundrobin", func() Selector { return new(RoundRobin) }) })
		assert.NotPanics(t, func() { RegisterSelector("testSelector", func() Selector { return new(testSelector) }) })
	})
	t.Run("getNewSelectorFunction", func(t *testing.T) {
		assert.True(t, HasSupportMethod("hash"))
		assert.NotNil(t, getNewSelectorFunction("hash"))

		assert.False(t, HasSupportMethod("invalid"))
		assert.NotNil(t, getNewSelectorFunction("invalid"))
	})
	t.Run("Methods", func(t *testing.T) {
		Methods()
	})
}

func TestBalanced(t *testing.T) {
	// start a server 1
	ln1, err := net.Listen("tcp", ":")
	require.NoError(t, err)
	go func() {
		for {
			_, err := ln1.Accept()
			if err != nil {
				return
			}
		}
	}()
	defer ln1.Close() // nolint: errcheck

	// start a server 2
	ln2, err := net.Listen("tcp", ":")
	require.NoError(t, err)
	go func() {
		for {
			_, err := ln2.Accept()
			if err != nil {
				return
			}
		}
	}()
	defer ln2.Close() // nolint: errcheck

	lnAddr1, lnAddr2 := ln1.Addr().String(), ln2.Addr().String()

	cfg := []Config{
		{
			Addr:   lnAddr1,
			Weight: 2,
		},
		{
			Addr:   lnAddr2,
			Weight: 4,
		},
	}

	lbWeight := New("weight", cfg,
		WithEnableDebug(false),
		WithLogger(logger.NewDiscard()),
		WithDNSServer(idns.New("8.8.8.8:53", 30)),
		WithGPool(nil),
		WithInterval(time.Millisecond*500))

	defer lbWeight.Close() // nolint: errcheck
	defer lbWeight.Close() // nolint: errcheck

	time.Sleep(time.Second * 3)

	assert.True(t, lbWeight.HasHealthy())
	assert.Equal(t, 2, lbWeight.HealthyCount())

	wantAddr := []string{lnAddr2, lnAddr1, lnAddr2, lnAddr2, lnAddr1, lnAddr2}

	for i := 0; i < len(wantAddr); i++ {
		addr := wantAddr[i%len(wantAddr)]
		lbWeight.ConnsIncrease(addr)
		assert.Equal(t, addr, lbWeight.Select(""))
	}

	for i := 0; i < len(wantAddr); i++ {
		addr := wantAddr[i%len(wantAddr)]
		lbWeight.ConnsDecrease(addr)
		assert.Equal(t, addr, lbWeight.Select(""))
	}

	lbWeight.debug = true
	assert.Equal(t, lnAddr2, lbWeight.Select(""))
	lbWeight.debug = false

	lbWeight.Reset(cfg[:1])
	assert.Equal(t, lnAddr1, lbWeight.Select(""))

	lbWeight.Reset(cfg[:0])
	assert.Equal(t, "", lbWeight.Select(""))
}
