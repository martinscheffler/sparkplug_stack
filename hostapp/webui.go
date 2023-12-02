package main

import (
	"log"
	"net/http"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	// Get the query parameter from the request
	name := r.URL.Query().Get("name")

	// Set the content type header
	w.Header().Set("Content-Type", "text/plain")

	// Write the response
	if len(name) > 0 {
		log.Printf("Hello, %s!", name)
	} else {
		log.Printf("Hello, World!")
	}
}

func startWebUI() error {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	// Handle dynamic requests
	http.HandleFunc("/hello", helloHandler)

	// Start the server
	log.Printf("Server listening on port 8080...")
	return http.ListenAndServe(":8080", nil)
}
