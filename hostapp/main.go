package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

var (
	natsBroker  string
	postgresURL string
)

var TemplateFileName string = "insert_spb.sql.gohtml"

func getEnvOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func init() {
	flag.StringVar(&natsBroker, "natsBroker", getEnvOrDefault("NATS_BROKER", "nats://127.0.0.1:4222"), "NATS Broker URL")
	flag.StringVar(&postgresURL, "postgresURL", getEnvOrDefault("POSTGRES_URL", "postgres://postgres:changeme@127.0.0.1:5432/hostapp"), "PostgreSQL URL")
	flag.Parse()
}

func waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
}

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

func main() {

	err := loadTemplateFromFile()
	if err != nil {
		log.Fatal(err)
	}

	err = connectDB(postgresURL)
	if err != nil {
		log.Fatal(err)
	}

	err = connectNats()
	if err != nil {
		log.Fatal(err)
	}

	err = startWebUI()
	if err != nil {
		log.Fatal(err)
	}

	waitForSignal()

	err = disconnectNats()
	if err != nil {
		log.Fatal(err)
	}

	err = disconnectDB()
	if err != nil {
		log.Fatal(err)
	}
}
