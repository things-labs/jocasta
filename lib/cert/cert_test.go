package cert

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateCA(t *testing.T) {
	err := CreateCAFile("ca", Config{
		Names: Names{
			Country: "CN",
		},
		Expire: 365 * 24,
	})
	require.NoError(t, err)

	ca, key, err := ParseCrtAndKeyFile("ca.crt", "ca.key")
	require.NoError(t, err)
	require.Equal(t, "CN", ca.Subject.Country[0])

	err = key.Validate()
	require.NoError(t, err)

	caBytes, keyBytes, err := ReadCrtAndKeyFile("ca.crt", "ca.key")
	require.NoError(t, err)

	ca, key, err = ParseCrtAndKey(caBytes, keyBytes)
	require.NoError(t, err)
	require.Equal(t, "CN", ca.Subject.Country[0])

	err = key.Validate()
	require.NoError(t, err)

	caBytes, keyBytes, err = ParseTLS("ca.crt", "ca.key")
	require.NoError(t, err)

	ca, key, err = ParseCrtAndKey(caBytes, keyBytes)
	require.NoError(t, err)
	require.Equal(t, "CN", ca.Subject.Country[0])

	err = key.Validate()
	require.NoError(t, err)

	os.Remove("ca.crt")
	os.Remove("ca.key")
}

func TestSign(t *testing.T) {
	err := CreateCAFile("ca", Config{
		CommonName: "server",
		Names: Names{
			Country:      "CN",
			Organization: "test",
		},
		Expire: 365 * 24,
	})
	require.NoError(t, err)

	ca, key, err := ParseCrtAndKeyFile("ca.crt", "ca.key")
	require.NoError(t, err)

	err = CreateSignFile(ca, key, "server", Config{
		CommonName: "server.com",
		Host:       []string{"server.com"},
		Names: Names{
			Country:      "CN",
			Organization: "test",
		},
		Expire: 365 * 24,
	})
	require.NoError(t, err)

	srvCa, srvKey, err := ParseCrtAndKeyFile("server.crt", "server.key")
	require.NoError(t, err)
	require.Equal(t, "server.com", srvCa.Subject.CommonName)

	err = srvKey.Validate()
	require.NoError(t, err)
	os.Remove("ca.crt")
	os.Remove("ca.key")
	os.Remove("server.crt")
	os.Remove("server.key")
}
