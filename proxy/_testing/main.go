package main

import (
	"log"
	"time"
)

func main() {
	a := make(chan struct{})
	for {
		select {
		case <-a:
			log.Println("closed")
			time.Sleep(time.Second)
		default:
			close(a)
			log.Println("default")
		}
	}

}
