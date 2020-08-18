package connection

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	t.Run("gc", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		mag := New(time.Millisecond*100, func(key string, value interface{}, now time.Time) bool {
			return key == "bar"
		})
		go mag.Watch(ctx)
		mag.Set("foo", "fooValue")
		mag.Set("bar", "barValue")
		mag.Set("car", "carValue")

		time.Sleep(time.Millisecond * 500)

		_, ok := mag.Get("bar")
		require.False(t, ok)

		v, ok := mag.Get("foo")
		require.True(t, ok)
		assert.Equal(t, "fooValue", v.(string))

		v, ok = mag.Get("car")
		require.True(t, ok)
		assert.Equal(t, "carValue", v.(string))

		cancel()

		<-ctx.Done()
	})

	t.Run("improve coverage", func(t *testing.T) {
		mag := New(-1, nil)
		mag.Watch(context.Background())
	})
}
