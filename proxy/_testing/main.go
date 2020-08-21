package main

import (
	"log"
	"time"
)

func main() {
	t := time.AfterFunc(time.Second, func() {
		log.Println(time.Now())
	})
	time.Sleep(time.Second * 2)
	<-t.C
}
