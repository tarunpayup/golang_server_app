package main

import (
	"log"
	"os"
)

func main() {

	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	log.Printf("CAOS server starting on port %s", port)

	if err := StartLoginServer(port); err != nil {
		log.Fatal(err)
	}
}
