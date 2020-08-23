package main

import (
	"log"
	"time"

	"github.com/thinkgos/jocasta/cs"
)

func main() {
	dial := cs.TCPDialer{
		Timeout:          time.Second * 5,
		AfterAdornChains: cs.AdornConnsChain{cs.AdornCzlib(true)},
	}
	conn, err := dial.Dial("tcp", "f.fjbjxdl.com:55555")
	if err != nil {
		panic(err)
	}
	rd := make([]byte, 1024)
	for {
		n, err := conn.Read(rd)
		if err != nil {
			log.Println(err)
			continue
		}
		log.Println(string(rd[:n]))
	}
}
