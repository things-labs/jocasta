package basicAuth

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thinkgos/jocasta/core/idns"
)

var testURL = "localhost"
var testPORT = 18888

func TestCenter(t *testing.T) {
	addr := ":" + strconv.Itoa(testPORT)

	c1 := New(WithAuthURL(testURL+addr, time.Millisecond*500, 204, 3),
		WithDNSServer(idns.New("127.0.0.1:53", 30)))
	_, err := c1.LoadFromFile("./testdata/nofile.txt")
	require.Error(t, err)

	cnt, err := c1.LoadFromFile("./testdata/userpassword.txt")
	require.NoError(t, err)
	assert.Equal(t, 4, cnt)

	cnt = c1.Add("xiaoxiao:123456", "xiaoju:123456")
	assert.Equal(t, 2, cnt)

	c1.Delete("xiaoxiao")
	assert.Equal(t, 5, c1.Total())
	assert.False(t, c1.Has("xiaoxiao"))
	assert.True(t, c1.Has("xiaoju"))
	assert.True(t, c1.VerifyFromLocal("xiaoju", "123456"))

	c2 := New()
	err = c2.VerifyFromURL("username", "password", "127.99.99.99", "127.6.6.6", "target")
	require.Error(t, err)
	c2.SetAuthURL(testURL+addr+"/invalid?a=b", time.Millisecond*500, 204, 3)
	err = c2.VerifyFromURL("username", "password", "127.99.99.99", "127.6.6.6", "target")
	require.Error(t, err)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		assert.Equal(t, "username", query.Get("user"))
		assert.Equal(t, "password", query.Get("pass"))
		assert.Equal(t, "127.99.99.99", query.Get("ip"))
		assert.Equal(t, "127.6.6.6", query.Get("local_ip"))
		assert.Equal(t, "target", query.Get("target"))
		w.WriteHeader(204)
	})
	http.HandleFunc("/invalid", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	})
	go http.ListenAndServe(addr, nil) // nolint: errcheck

	time.Sleep(time.Second)

	err = c2.VerifyFromURL("username", "password", "127.99.99.99", "127.6.6.6", "target")
	require.Error(t, err)

	err = c1.VerifyFromURL("username", "password", "127.99.99.99", "127.6.6.6", "target")
	require.NoError(t, err)

	ok := c1.Verify(Format("username", "password"), "127.99.99.99", "127.6.6.6", "target")
	require.True(t, ok)
	// invalid user password pair
	ok = c1.Verify("invalid", "127.99.99.99", "127.6.6.6", "target")
	require.False(t, ok)
}
