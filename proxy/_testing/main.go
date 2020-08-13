package main

import (
	"net/http"
)

func main() {
	srv := http.Server{}
	srv.Serve()
	srv.Close()
}
