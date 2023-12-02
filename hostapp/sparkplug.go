package main

import (
	"errors"
	"google.golang.org/protobuf/proto"
	"hostapp/sparkplug_b"
	"strings"
)

type SparkplugMessage struct {
	Namespace   string
	GroupId     string
	MessageType string
	EdgeNodeId  string
	DeviceId    string
	Payload     *sparkplug_b.Payload
}

func decodeSparkplugMessage(subject string, data []byte) (*SparkplugMessage, error) {
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
