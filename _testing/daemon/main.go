package main

import (
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/takama/daemon"
)

const (
	name        = "opp"
	description = "opp Service"
)

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	srv, err := daemon.New(name, description)
	handleErr(err)

	service := Service{srv}

	s, err := service.Manage()
	if err != nil {
		log.Println(s)
	} else {
		log.Println(s)
	}
}

// Service has embedded daemon
type Service struct {
	daemon.Daemon
}

// Manage by daemon commands or run the daemon
func (sf *Service) Manage() (string, error) {

	usage := "Usage: opp install | remove | start | stop | status"

	// if received any kind of command, do it
	if len(os.Args) > 1 {
		command := os.Args[1]
		switch command {
		case "install":
			return sf.Install()
		case "remove":
			return sf.Remove()
		case "start":
			return sf.Start()
		case "stop":
			return sf.Stop()
		case "status":
			return sf.Status()
		default:
			return usage, nil
		}
	}
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "helloworld: "+strconv.Itoa(rand.Int()))
		})
		http.ListenAndServe(":8080", nil)
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)

	<-interrupt
	log.Println("stopped")
	return "", nil
}
