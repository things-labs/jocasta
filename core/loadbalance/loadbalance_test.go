package loadbalance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testSelector struct{}

func (testSelector) Select(UpstreamPool, string) *Upstream {
	return nil
}

func TestRegisterSelector(t *testing.T) {
	assert.Panics(t, func() { RegisterSelector("", nil) })
	assert.Panics(t, func() { RegisterSelector("", func() Selector { return nil }) })
	assert.Panics(t, func() { RegisterSelector("roundrobin", func() Selector { return new(RoundRobin) }) })
	assert.NotPanics(t, func() { RegisterSelector("testSelector", func() Selector { return new(testSelector) }) })
}

func Test_getNewSelectorFunction(t *testing.T) {
	assert.True(t, HasSupportMethod("hash"))
	assert.NotNil(t, getNewSelectorFunction("hash"))

	assert.False(t, HasSupportMethod("invalid"))
	assert.NotNil(t, getNewSelectorFunction("invalid"))
}
