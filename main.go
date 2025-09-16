package main

import (
	"log"
	"net/http"
)

func main() {
	LoadData()
	srv := RegisterHandlers()
	log.Printf("server listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
