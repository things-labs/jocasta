package idns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdns(t *testing.T) {
	publicDNSAddr := "223.5.5.5:53"
	// 阿里云DNS服务 223.5.5.5;223.6.6.6
	srv := New(publicDNSAddr, 3600)
	assert.Equal(t, 3600, srv.TTL())
	assert.Equal(t, publicDNSAddr, srv.PublicDNSAddr())

	domain := "www.baidu.com"
	ip, err := srv.Resolve(domain)
	require.NoError(t, err)
	t.Logf("resolve domain: %s - %s", domain, ip)
	ip = srv.MustResolve(domain)
	t.Logf("MustResolve domain: %s - %s", domain, ip)

	domainWithPort := "www.baidu.com:80"
	ip, err = srv.Resolve(domainWithPort)
	require.NoError(t, err)
	t.Logf("resolve domain: %s - %s", domainWithPort, ip)

	domainButIp := "127.0.0.1"
	ip, err = srv.Resolve(domainButIp)
	require.NoError(t, err)
	t.Logf("resolve domain: %s - %s", domainButIp, ip)

	domainButIpPort := "127.0.0.1:80"
	ip, err = srv.Resolve(domainButIpPort)
	require.NoError(t, err)
	t.Logf("resolve domain: %s - %s", domainButIpPort, ip)
}
