package lb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testUpstreamPool() UpstreamPool {
	pool := make([]*Upstream, 0, 3)
	pool = append(pool, &Upstream{active: 1})
	pool = append(pool, &Upstream{active: 1})
	pool = append(pool, &Upstream{active: 1})
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
	pool[1].active = 0
	h = robin.Select(pool, "")
	assert.NotEqual(t, pool[1], h, "Expected to skip down host.")

	// mark host as up
	pool[1].active = 1

	h = robin.Select(pool, "")
	assert.Equal(t, pool[2], h, "Expected to balance evenly among healthy hosts")

	// mark host as full
	// pool[1].CountRequest(1)
	// pool[1].MaxRequests = 1
	// h = robin.Select(pool, "")
	// assert.NotEqual(t, pool[2], h, "Expected to skip full host.")
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
	pool[1].active = 0
	h = ipHash.Select(pool, "172.0.0.1")
	assert.Equal(t, pool[2], h, "Expected ip hash policy host to be the third host.")

	h = ipHash.Select(pool, "172.0.0.2")
	assert.Equal(t, pool[2], h, "Expected ip hash policy host to be the third host.")

	pool[1].active = 1

	pool[2].active = 0
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
	pool[0].active = 0
	pool[1].active = 0
	h = ipHash.Select(pool, "172.0.0.4:80")
	assert.Nil(t, h, "Expected ip hash policy host to be the second host.")
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
	pool[1].active = 0
	h = addrHash.Select(pool, "172.0.0.1")
	assert.Equal(t, pool[2], h, "Expected ip hash policy host to be the third host.")

	h = addrHash.Select(pool, "172.0.0.2")
	assert.Equal(t, pool[2], h, "Expected ip hash policy host to be the third host.")

	pool[1].active = 1

	pool[2].active = 0
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
	pool[0].active = 0
	pool[1].active = 0
	h = addrHash.Select(pool, "172.0.0.4:80")
	assert.Nil(t, h, "Expected ip hash policy host to be the second host.")
}
