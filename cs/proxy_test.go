package cs

import (
	"testing"
)

func TestValidJumperProxyURL(t *testing.T) {
	tests := []struct {
		name     string
		proxyURL string
		want     bool
	}{
		{"https://host:8080", "https://host:8080", true},
		{"https://username:password@host:8080", "https://username:password@host:8080", true},
		{"socks5://host:8080", "socks5://host:8080", true},
		{"socks5://username:password@host:8080", "socks5://username:password@host:8080", true},
		{"invalid", "invalid", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidProxyURL(tt.proxyURL); got != tt.want {
				t.Errorf("ValidProxyURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
