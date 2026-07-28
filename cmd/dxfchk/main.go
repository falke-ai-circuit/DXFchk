package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/falke-ai-circuit/DXFchk/internal/api"
)

var version = "v0.1.0"

func main() {
	port := flag.Int("port", 8643, "HTTP server port")
	flag.Parse()

	server := api.NewServer()

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("DXFchk %s starting on http://localhost%s", version, addr)
	log.Printf("API: http://localhost:%d/api/v1/health", *port)

	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}