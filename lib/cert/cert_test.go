package cert

import (
	"os"
	"testing"
)

func TestGenerateCA(t *testing.T) {
	err := CreateCAFile("ca", Config{
		Names: Names{
			Country: "CN",
		},
		Expire: 365 * 24,
	})
	if err != nil {
		t.Fatal(err)
		return
	}
	ca, key, err := ParseCrtAndKeyFile("ca.crt", "ca.key")
	if err != nil {
		t.Fatal(err)
		return
	}
	if ca.Subject.Country[0] != "CN" {
		t.Fatalf("Country %s not match test", ca.Subject.Country[0])
		return
	}
	err = key.Validate()
	if err != nil {
		t.Fatal(err)
		return
	}
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
	if err != nil {
		t.Fatal(err)
		return
	}
	ca, key, err := ParseCrtAndKeyFile("ca.crt", "ca.key")
	if err != nil {
		t.Fatal(err)
		return
	}
	err = CreateSignFile(ca, key, "server", Config{
		CommonName: "server.com",
		Host:       []string{"server.com"},
		Names: Names{
			Country:      "CN",
			Organization: "test",
		},
		Expire: 365 * 24,
	})
	if err != nil {
		t.Fatal(err)
		return
	}
	srvCa, srvKey, err := ParseCrtAndKeyFile("server.crt", "server.key")
	if err != nil {
		t.Fatal(err)
		return
	}
	if srvCa.Subject.CommonName != "server.com" {
		t.Fatalf("CommonName %s not match server.com", ca.Subject.CommonName)
		return
	}
	err = srvKey.Validate()
	if err != nil {
		t.Fatal(err)
		return
	}
	os.Remove("ca.crt")
	os.Remove("ca.key")
	os.Remove("server.crt")
	os.Remove("server.key")
}
