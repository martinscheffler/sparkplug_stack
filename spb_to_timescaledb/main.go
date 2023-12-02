package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v4"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"gosp3/sparkplug_b"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/template"
)

var sqlTemplate *template.Template

var (
	NatsBroker  string
	PostgresURL string
)

var PgCon *pgx.Conn
var PgCtx = context.Background()

var NatsCon *nats.Conn
var NatsSub *nats.Subscription

var TemplateFileName string = "insert_spb.sql.gohtml"

func getEnvOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func init() {
	flag.StringVar(&NatsBroker, "NatsBroker", getEnvOrDefault("NATS_BROKER", "nats://127.0.0.1:4222"), "NATS Broker URL")
	flag.StringVar(&PostgresURL, "PostgresURL", getEnvOrDefault("POSTGRES_URL", "postgres://postgres:changeme@127.0.0.1:5432/gosp3"), "PostgreSQL URL")
	flag.Parse()
}

type SparkplugMessage struct {
	Namespace   string
	GroupId     string
	MessageType string
	EdgeNodeId  string
	DeviceId    string
	Payload     *sparkplug_b.Payload
}

func NewSparkplugMessage(subject string, data []byte) (*SparkplugMessage, error) {
	parts := strings.Split(subject, ".")
	if len(parts) < 4 {
		return nil, errors.New("expected at least 4 parts in message subject")
	}

	deviceId := ""
	if len(parts) > 4 {
		deviceId = parts[4]
	}
	var payload sparkplug_b.Payload
	// Unmarshal the byte stream into the sparkplug B payload struct
	err := proto.Unmarshal(data, &payload)
	if err != nil {
		return nil, err
	}

	return &SparkplugMessage{
		Namespace:   parts[0],
		GroupId:     parts[1],
		MessageType: parts[2],
		EdgeNodeId:  parts[3],
		DeviceId:    deviceId,
		Payload:     &payload,
	}, nil
}

func storeSparkplugMessageToDB(sparkplugMessage *SparkplugMessage) error {

	var buffer bytes.Buffer
	err := sqlTemplate.Execute(&buffer, sparkplugMessage)
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	sql := buffer.String()
	_, err = PgCon.Exec(PgCtx, sql)
	if err != nil {
		fmt.Println("Error:", err)
		fmt.Println("SQL:", sql)
	}
	return err
}

func onReceive(msg *nats.Msg) {
	sparkplugMsg, err := NewSparkplugMessage(msg.Subject, msg.Data)
	if err != nil {
		fmt.Println("Error during unmarshalling:", err)
		return
	}
	log.Printf("Received: %s", string(msg.Subject))
	err = storeSparkplugMessageToDB(sparkplugMsg)
	if err != nil {
		fmt.Println("Error saving to DB:", err)
		return
	}
}

func loadTemplateFromFile() {
	// load template from file
	t, err := template.New(TemplateFileName).ParseFiles(TemplateFileName)
	if err != nil {
		panic(err)
	}
	sqlTemplate = t
}

func connectDB() {
	fmt.Printf("Connecting to PostgreSQL on URL %v.\n", PostgresURL)
	pgConfig, err := pgx.ParseConfig(PostgresURL)
	if err != nil {
		log.Fatal("error parsing postgres config: ", err)
	}

	PgCon, err = pgx.ConnectConfig(PgCtx, pgConfig)
	if err != nil {
		log.Fatal("unable to connect to database: ", err)
	}
	log.Println("Connected to TimescaleDB.")
}

func disconnectDB() {
	PgCon.Close(PgCtx)
}

func connectNats() {

	fmt.Printf("Connecting to NATS on URL %v.\n", NatsBroker)
	// connect to NATS
	nc, err := nats.Connect(NatsBroker)
	if err != nil {
		log.Fatal(err)
	}
	NatsCon = nc
	log.Println("Connected to NATS.")

	// Subscribe to all MQTT topics starting with "spBv1.0/"
	sub, err := nc.Subscribe("spBv1//0.>", onReceive)
	if err != nil {
		log.Fatal(err)
	}
	NatsSub = sub
}

func disconnectNats() {
	NatsSub.Unsubscribe()
	NatsCon.Close()
}

func waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
}

func main() {

	loadTemplateFromFile()
	connectDB()
	connectNats()

	waitForSignal()

	disconnectNats()
	disconnectDB()
}
