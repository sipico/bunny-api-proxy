package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

const version = "0.1.0"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/", rootHandler)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Bunny API Proxy v%s starting on %s", version, addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","version":"%s"}`, version)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"message":"Bunny API Proxy","version":"0.1.0"}`)
}
