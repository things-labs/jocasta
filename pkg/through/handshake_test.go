package through

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thinkgos/jocasta/pkg/through/ddt"
)

func TestHandshakeRequest(t *testing.T) {
	nego := &HandshakeRequest{
		Version,
		ddt.HandshakeRequest{
			NodeId:    "NodeId",
			SessionId: "SessionId",
			Protocol:  ddt.Network_TCP,
			Host:      "localhost",
			Port:      8080,
		},
	}

	b, err := nego.Bytes()
	require.NoError(t, err)

	want, err := ParseHandshakeRequest(bytes.NewReader(b))
	require.NoError(t, err)

	assert.Equal(t, nego.Version, want.Version)
	assert.Equal(t, nego.Hand.NodeId, want.Hand.NodeId)
	assert.Equal(t, nego.Hand.SessionId, want.Hand.SessionId)
	assert.Equal(t, nego.Hand.Protocol, want.Hand.Protocol)
	assert.Equal(t, nego.Hand.Host, want.Hand.Host)
	assert.Equal(t, nego.Hand.Port, want.Hand.Port)
}
