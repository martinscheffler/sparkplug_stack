package main

import (
	"flag"
	"log"
	"os"
)

var (
	natsBroker  string
	postgresURL string
)

var templateFileName = "insert_spb.sql.gohtml"

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

func main() {

	err := loadSparkplugSqlTemplateFromFile(templateFileName)
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

	startWebUI()

	err = disconnectNats()
	if err != nil {
		log.Fatal(err)
	}

	err = disconnectDB()
	if err != nil {
		log.Fatal(err)
	}
}
