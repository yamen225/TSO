package main

import (
	"log"
	"net/http"

	"activation-service/internal/handler"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/optimize", handler.OptimizeHandler)

	log.Println("Starting activation service on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
