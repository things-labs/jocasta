package cert

import (
	"encoding/base64"
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

	// invalid ca file
	_, _, err = ParseCrtAndKeyFile("invalid.crt", "ca.key")
	require.Error(t, err)

	// invalid key file
	_, _, err = ParseCrtAndKeyFile("ca.crt", "invalid.key")
	require.Error(t, err)

	// ca key file
	ca, key, err := ParseCrtAndKeyFile("ca.crt", "ca.key")
	require.NoError(t, err)
	require.Equal(t, "CN", ca.Subject.Country[0])

	err = key.Validate()
	require.NoError(t, err)

	// invalid ca file
	_, _, err = LoadCrtAndKeyFile("invalid.crt", "ca.key")
	require.Error(t, err)

	// invalid key file
	_, _, err = LoadCrtAndKeyFile("ca.crt", "invalid.key")
	require.Error(t, err)

	// ca key file
	caBytes, keyBytes, err := LoadCrtAndKeyFile("ca.crt", "ca.key")
	require.NoError(t, err)

	ca, key, err = ParseCrtAndKey(caBytes, keyBytes)
	require.NoError(t, err)
	require.Equal(t, "CN", ca.Subject.Country[0])

	err = key.Validate()
	require.NoError(t, err)

	// file
	caBytes, keyBytes, err = Load("ca.crt", "ca.key")
	require.NoError(t, err)

	_, _, err = ParseCrtAndKey(caBytes, keyBytes)
	require.NoError(t, err)
	require.Equal(t, "CN", ca.Subject.Country[0])

	// base64 string
	caStr := base64Prefix + base64.StdEncoding.EncodeToString(caBytes)
	keyStr := base64Prefix + base64.StdEncoding.EncodeToString(keyBytes)
	caBytes, keyBytes, err = Load(caStr, keyStr)
	require.NoError(t, err)

	ca, key, err = ParseCrtAndKey(caBytes, keyBytes)
	require.NoError(t, err)
	require.Equal(t, "CN", ca.Subject.Country[0])

	// invalid base64 string
	_, _, err = Load(base64Prefix+"invalidbase64", base64Prefix+"invalidbase64")
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
