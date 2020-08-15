package loadbalance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testUpstreamPool() UpstreamPool {
	pool := make([]*Upstream, 0, 3)
	pool = append(pool, &Upstream{health: 1})
	pool = append(pool, &Upstream{health: 1})
	pool = append(pool, &Upstream{health: 1})
	return pool
}

func TestRandom_Select(t *testing.T) {
	upool := testUpstreamPool()
	rd := new(Random)

	require.NotNil(t, rd.Select(upool, ""))
}

func TestRoundRobin_Select(t *testing.T) {
	pool := testUpstreamPool()
	robin := new(RoundRobin)

	// First selected host is 1, because counter starts at 0
	// and increments before host is selected
	h := robin.Select(pool, "")
	assert.Equal(t, pool[1], h)
	h = robin.Select(pool, "")
	assert.Equal(t, pool[2], h)
	h = robin.Select(pool, "")
	assert.Equal(t, pool[0], h)

	// mark host as down
	pool[1].health = 0
	h = robin.Select(pool, "")
	assert.NotEqual(t, pool[1], h, "Expected to skip down host.")

	// mark host as up
	pool[1].health = 1

	h = robin.Select(pool, "")
	assert.Equal(t, pool[2], h, "Expected to balance evenly among healthy hosts")

	// mark host as full
	pool[1].MaxConnections = 1
	pool[1].connections = 1
	h = robin.Select(pool, "")
	assert.Equal(t, pool[2], h, "Expected to skip full host.")

	// improve code coverage
	robin.Select(pool[:0], "")
}

func TestLeastConn_Select(t *testing.T) {
	pool := testUpstreamPool()
	lcPolicy := new(LeastConn)

	pool[0].connections = 10
	pool[1].connections = 10
	h := lcPolicy.Select(pool, "")
	assert.Equal(t, pool[2], h, "Expected least connection host to be third host.")

	pool[2].connections = 100
	h = lcPolicy.Select(pool, "")
	assert.Contains(t, []*Upstream{pool[0], pool[1]}, h, "Expected least connection host to be first or second host.")
}

func TestIPHash_Select(t *testing.T) {
	pool := testUpstreamPool()
	ipHash := new(IPHash)

	// We should be able to predict where every request is routed.
	h := ipHash.Select(pool, "172.0.0.1:80")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	h = ipHash.Select(pool, "172.0.0.2:80")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	h = ipHash.Select(pool, "172.0.0.3:80")
	assert.Equal(t, pool[2], h, "Expected ip hash policy host to be the third host.")

	h = ipHash.Select(pool, "172.0.0.4:80")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	// we should get the same results without a port
	h = ipHash.Select(pool, "172.0.0.1")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	h = ipHash.Select(pool, "172.0.0.2")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	h = ipHash.Select(pool, "172.0.0.3")
	assert.Equal(t, pool[2], h, "Expected ip hash policy host to be the third host.")

	h = ipHash.Select(pool, "172.0.0.4")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	// we should get a healthy host if the original host is unhealthy and a
	// healthy host is available
	pool[1].health = 0
	h = ipHash.Select(pool, "172.0.0.1")
	assert.Equal(t, pool[2], h, "Expected ip hash policy host to be the third host.")

	h = ipHash.Select(pool, "172.0.0.2")
	assert.Equal(t, pool[2], h, "Expected ip hash policy host to be the third host.")

	pool[1].health = 1

	pool[2].health = 0
	h = ipHash.Select(pool, "172.0.0.3")
	assert.Equal(t, pool[0], h, "Expected ip hash policy host to be the first host.")

	h = ipHash.Select(pool, "172.0.0.4")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	// We should be able to resize the host pool and still be able to predict
	// where a req will be routed with the same IP's used above
	pool = pool[:2]

	h = ipHash.Select(pool, "172.0.0.1:80")
	assert.Equal(t, pool[0], h, "Expected ip hash policy host to be the first host.")

	h = ipHash.Select(pool, "172.0.0.2:80")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	h = ipHash.Select(pool, "172.0.0.3:80")
	assert.Equal(t, pool[0], h, "Expected ip hash policy host to be the first host.")

	h = ipHash.Select(pool, "172.0.0.4:80")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	// We should get nil when there are no healthy hosts
	pool[0].health = 0
	pool[1].health = 0
	h = ipHash.Select(pool, "172.0.0.4:80")
	assert.Nil(t, h, "Expected ip hash policy host to be the second host.")

	// improve code coverage
	ipHash.Select(pool, "")
}

func TestAddrHash_Select(t *testing.T) {
	pool := testUpstreamPool()
	addrHash := new(AddrHash)

	// We should be able to predict where every request is routed.
	h := addrHash.Select(pool, "172.0.0.1:80")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	h = addrHash.Select(pool, "172.0.0.2:80")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	h = addrHash.Select(pool, "172.0.0.3:80")
	assert.Equal(t, pool[2], h, "Expected ip hash policy host to be the third host.")

	h = addrHash.Select(pool, "172.0.0.4:80")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	// we should get the same results without a port
	h = addrHash.Select(pool, "172.0.0.1")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	h = addrHash.Select(pool, "172.0.0.2")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	h = addrHash.Select(pool, "172.0.0.3")
	assert.Equal(t, pool[2], h, "Expected ip hash policy host to be the third host.")

	h = addrHash.Select(pool, "172.0.0.4")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	// we should get a healthy host if the original host is unhealthy and a
	// healthy host is available
	pool[1].health = 0
	h = addrHash.Select(pool, "172.0.0.1")
	assert.Equal(t, pool[2], h, "Expected ip hash policy host to be the third host.")

	h = addrHash.Select(pool, "172.0.0.2")
	assert.Equal(t, pool[2], h, "Expected ip hash policy host to be the third host.")

	pool[1].health = 1

	pool[2].health = 0
	h = addrHash.Select(pool, "172.0.0.3")
	assert.Equal(t, pool[0], h, "Expected ip hash policy host to be the first host.")

	h = addrHash.Select(pool, "172.0.0.4")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	// We should be able to resize the host pool and still be able to predict
	// where a req will be routed with the same IP's used above
	pool = pool[:2]

	h = addrHash.Select(pool, "172.0.0.1:80")
	assert.Equal(t, pool[0], h, "Expected ip hash policy host to be the first host.")

	h = addrHash.Select(pool, "172.0.0.2:80")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	h = addrHash.Select(pool, "172.0.0.3:80")
	assert.Equal(t, pool[0], h, "Expected ip hash policy host to be the first host.")

	h = addrHash.Select(pool, "172.0.0.4:80")
	assert.Equal(t, pool[1], h, "Expected ip hash policy host to be the second host.")

	// We should get nil when there are no healthy hosts
	pool[0].health = 0
	pool[1].health = 0
	h = addrHash.Select(pool, "172.0.0.4:80")
	assert.Nil(t, h, "Expected ip hash policy host to be the second host.")

	// improve code coverage
	addrHash.Select(pool, "")
}

func TestWeight_Select(t *testing.T) {
	pool := make([]*Upstream, 0, 3)
	pool = append(pool, &Upstream{health: 1, Config: Config{Addr: ":80", Weight: 3}})
	pool = append(pool, &Upstream{health: 1, Config: Config{Addr: ":81", Weight: 2}})
	pool = append(pool, &Upstream{health: 1, Config: Config{Addr: ":82", Weight: 4}})

	wantAddr := []string{":82", ":80", ":82", ":80", ":81", ":82", ":80", ":81", ":82"}

	wei := NewWeight()

	for i := 0; i < 18; i++ {
		assert.Equal(t, wantAddr[i%len(wantAddr)], wei.Select(pool, "").Addr)
	}

	assert.Equal(t, ":80", wei.Select(pool[:1], "").Addr)
	assert.Nil(t, wei.Select(pool[:0], ""))
}

func TestLeastTime_Select(t *testing.T) {
	pool := testUpstreamPool()
	sel := new(LeastTime)

	pool[0].leastTime.Store(time.Second)
	pool[1].leastTime.Store(time.Second)
	pool[2].leastTime.Store(time.Millisecond)
	h := sel.Select(pool, "")
	assert.Equal(t, pool[2], h, "Expected least connection host to be third host.")

	pool[2].leastTime.Store(time.Second * 100)
	h = sel.Select(pool, "")
	assert.Contains(t, []*Upstream{pool[0], pool[1]}, h, "Expected least connection host to be first or second host.")
}
