package main

import (
	"github.com/nats-io/nats.go"
	"log"
)

var natsCon *nats.Conn
var natsSub *nats.Subscription

func connectNats() error {

	log.Printf("Connecting to NATS on URL %v.\n", natsBroker)
	// connect to NATS
	nc, err := nats.Connect(natsBroker)
	if err != nil {
		return err
	}
	natsCon = nc
	log.Println("Connected to NATS.")

	// Subscribe to all MQTT topics starting with "spBv1.0/"
	sub, err := nc.Subscribe("spBv1//0.>", onReceive)
	if err != nil {
		return err
	}
	natsSub = sub
	return nil
}

func disconnectNats() error {
	err := natsSub.Unsubscribe()
	if err != nil {
		return err
	}
	natsCon.Close()
	return nil
}

func onReceive(msg *nats.Msg) {
	sparkplugMsg, err := decodeSparkplugMessage(msg.Subject, msg.Data)
	if err != nil {
		log.Printf("Error during unmarshalling: %v", err)
		return
	}
	log.Printf("Received: %s Msg: %s", msg.Subject, sparkplugMsg)
	err = storeSparkplugMessageToDB(sparkplugMsg)
	if err != nil {
		log.Printf("Error saving to DB: %v", err)
		return
	}
}
