package basicAuth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCenter(t *testing.T) {
	c := New()
	_, err := c.LoadFromFile("./testdata/nofile.txt")
	require.Error(t, err)

	cnt, err := c.LoadFromFile("./testdata/userpassword.txt")
	require.NoError(t, err)
	assert.Equal(t, 4, cnt)

	cnt = c.Add("xiaoxiao:123456", "xiaoju:123456")
	assert.Equal(t, 2, cnt)
	c.Delete("xiaoxiao")

	assert.Equal(t, 5, c.Total())

	assert.True(t, c.Has("xiaoju"))
	assert.True(t, c.VerifyFromLocal("xiaoju", "123456"))
}
