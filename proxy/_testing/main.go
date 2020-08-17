package main

import (
	"log"

	"golang.org/x/net/proxy"

	"github.com/thinkgos/jocasta/cs"
)

func main() {
	a, err := cs.ParseProxyURL("")
	log.Printf("%v", err)
	log.Printf("%v", a)
	proxy.SOCKS5()
}
