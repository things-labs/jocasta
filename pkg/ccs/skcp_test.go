package ccs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSKcpMode(t *testing.T) {
	noDelay, interval, resend, noCongestion := SKcpMode("fast")
	assert.Equal(t, 0, noDelay)
	assert.Equal(t, 30, interval)
	assert.Equal(t, 2, resend)
	assert.Equal(t, 1, noCongestion)

	noDelay, interval, resend, noCongestion = SKcpMode("fast2")
	assert.Equal(t, 1, noDelay)
	assert.Equal(t, 20, interval)
	assert.Equal(t, 2, resend)
	assert.Equal(t, 1, noCongestion)

	noDelay, interval, resend, noCongestion = SKcpMode("fast3")
	assert.Equal(t, 1, noDelay)
	assert.Equal(t, 10, interval)
	assert.Equal(t, 2, resend)
	assert.Equal(t, 1, noCongestion)

	noDelay, interval, resend, noCongestion = SKcpMode("normal")
	assert.Equal(t, 0, noDelay)
	assert.Equal(t, 40, interval)
	assert.Equal(t, 2, resend)
	assert.Equal(t, 1, noCongestion)
}
