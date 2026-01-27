package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
)

func main() {
	port := flag.String("port", "8001", "Port to listen on")
	name := flag.String("name", "backend-1", "Service name")
	flag.Parse()
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hostname, _ := os.Hostname()
		response := map[string]string{
			"service":  *name,
			"hostname": hostname,
			"port":     *port,
			"message":  "Hello from " + *name,
			"path":     r.URL.Path,
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		
		log.Printf("Request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
	})
	
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	addr := ":" + *port
	log.Printf("Starting example backend service '%s' on %s", *name, addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
