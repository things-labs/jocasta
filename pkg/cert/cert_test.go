package cert

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thinkgos/jocasta/pkg/extcert"
)

func TestGenerateCA(t *testing.T) {
	err := CreateCAFile("ca", Config{
		Names: Names{
			Country: "CN",
		},
		Expire: 365 * 24,
	})
	require.NoError(t, err)

	// invalid ca file
	_, _, err = extcert.ParseCrtAndKeyFile("invalid.crt", "ca.key")
	require.Error(t, err)

	// invalid key file
	_, _, err = extcert.ParseCrtAndKeyFile("ca.crt", "invalid.key")
	require.Error(t, err)

	// ca key file
	ca, key, err := extcert.ParseCrtAndKeyFile("ca.crt", "ca.key")
	require.NoError(t, err)
	require.Equal(t, "CN", ca.Subject.Country[0])

	err = key.Validate()
	require.NoError(t, err)

	// invalid ca file
	_, _, err = extcert.LoadCrtAndKeyFile("invalid.crt", "ca.key")
	require.Error(t, err)

	// invalid key file
	_, _, err = extcert.LoadCrtAndKeyFile("ca.crt", "invalid.key")
	require.Error(t, err)

	// ca key file
	caBytes, keyBytes, err := extcert.LoadCrtAndKeyFile("ca.crt", "ca.key")
	require.NoError(t, err)

	ca, key, err = extcert.ParseCrtAndKey(caBytes, keyBytes)
	require.NoError(t, err)
	require.Equal(t, "CN", ca.Subject.Country[0])

	err = key.Validate()
	require.NoError(t, err)

	// file
	caBytes, keyBytes, err = extcert.LoadPair("ca.crt", "ca.key")
	require.NoError(t, err)

	_, _, err = extcert.ParseCrtAndKey(caBytes, keyBytes)
	require.NoError(t, err)
	require.Equal(t, "CN", ca.Subject.Country[0])

	// base64 string
	caStr := base64Prefix + base64.StdEncoding.EncodeToString(caBytes)
	keyStr := base64Prefix + base64.StdEncoding.EncodeToString(keyBytes)
	caBytes, keyBytes, err = extcert.LoadPair(caStr, keyStr)
	require.NoError(t, err)

	ca, key, err = extcert.ParseCrtAndKey(caBytes, keyBytes)
	require.NoError(t, err)
	require.Equal(t, "CN", ca.Subject.Country[0])

	// invalid base64 string
	_, _, err = extcert.LoadPair(base64Prefix+"invalidbase64", base64Prefix+"invalidbase64")
	require.Error(t, err)

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

	ca, key, err := extcert.ParseCrtAndKeyFile("ca.crt", "ca.key")
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

	srvCa, srvKey, err := extcert.ParseCrtAndKeyFile("server.crt", "server.key")
	require.NoError(t, err)
	require.Equal(t, "server.com", srvCa.Subject.CommonName)

	err = srvKey.Validate()
	require.NoError(t, err)
	os.Remove("ca.crt")
	os.Remove("ca.key")
	os.Remove("server.crt")
	os.Remove("server.key")
}
