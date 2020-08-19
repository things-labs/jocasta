package through

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thinkgos/jocasta/pkg/through/ddt"
)

func TestNegotiateRequest(t *testing.T) {
	nego := &NegotiateRequest{
		TypesClient,
		Version,
		ddt.NegotiateRequest{
			SecretKey: "SecretKey",
			Id:        "Id",
		},
	}

	b, err := nego.Bytes()
	require.NoError(t, err)

	want, err := ParseNegotiateRequest(bytes.NewReader(b))
	require.NoError(t, err)

	assert.Equal(t, nego.Types, want.Types)
	assert.Equal(t, nego.Version, want.Version)
	assert.Equal(t, nego.Nego.SecretKey, want.Nego.SecretKey)
	assert.Equal(t, nego.Nego.Id, want.Nego.Id)
}
