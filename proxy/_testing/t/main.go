// package main
//
// import (
// 	"log"
// 	"net"
// 	"time"
// )
//
// func main() {
// 	start := time.Now()
// 	a, err := net.ResolveUDPAddr("udp", ":8080")
// 	if err != nil {
// 		panic(err)
// 	}
// 	log.Println(a)
//
// 	// h, p, err := net.SplitHostPort(":8080")
// 	// if err != nil {
// 	// 	panic(err)
// 	// }
// 	// port, err := strconv.Atoi(p)
// 	// if err != nil {
// 	// 	panic(err)
// 	// }
// 	// log.Println(net.ParseIP(h), port)
// 	log.Println(time.Now().Sub(start).String())
// }

package main

import (
	"crypto/sha1"
	"io"
	"log"
	"time"

	"github.com/xtaci/kcp-go/v5"
	"golang.org/x/crypto/pbkdf2"
)

func main() {
	key := pbkdf2.Key([]byte("demo pass"), []byte("demo salt"), 1024, 32, sha1.New)
	block, _ := kcp.NewAESBlockCrypt(key)

	listener, err := kcp.ListenWithOptions("127.0.0.1:12345", block, 10, 3)
	if err != nil {
		log.Fatal(err)
	}
	// spin-up the client
	go func() {
		// wait for server to become ready
		time.Sleep(time.Second)

		// dial to the echo server
		sess, err := kcp.DialWithOptions("127.0.0.1:12345", block, 10, 3)
		if err != nil {
			log.Fatal(err)
		}
		defer sess.Close()
		for {
			data := time.Now().String()
			buf := make([]byte, len(data))
			log.Println("sent:", data)
			if _, err := sess.Write([]byte(data)); err == nil {
				// read back the data
				if _, err := io.ReadFull(sess, buf); err == nil {
					log.Println("recv:", string(buf))
				} else {
					log.Fatal(err)
				}
			} else {
				log.Fatal(err)
			}
			time.Sleep(time.Second * 7)
		}
	}()
	for {
		listener.SetDeadline(time.Now().Add(time.Second * 5))
		s, err := listener.AcceptKCP()
		listener.SetDeadline(time.Time{})
		if err != nil {
			log.Println(err)
			break
		}
		go handleEcho(s)
	}
	listener.Close()
	log.Println("out")
	select {}
}

// handleEcho send back everything it received
func handleEcho(conn *kcp.UDPSession) {
	// conn.SetDeadline()
	defer func() {
		log.Println("close")
		conn.Close()
	}()
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Println(err)
			return
		}

		n, err = conn.Write(buf[:n])
		if err != nil {
			log.Println(err)
			return
		}
	}
}
