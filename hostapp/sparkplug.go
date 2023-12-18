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

var DataTypes = map[int]string{
	0:  "Unknown",
	1:  "Int8",
	2:  "Int16",
	3:  "Int32",
	4:  "Int64",
	5:  "UInt8",
	6:  "UInt16",
	7:  "UInt32",
	8:  "UInt64",
	9:  "Float",
	10: "Double",
	11: "Boolean",
	12: "String",
	13: "DateTime",
	14: "Text",
	15: "UUID",
	16: "DataSet",
	17: "Bytes",
	18: "File",
	19: "Template",
	20: "PropertySet",
	21: "PropertySetList",
	22: "Int8Array",
	23: "Int16Array",
	24: "Int32Array",
	25: "Int64Array",
	26: "UInt8Array",
	27: "UInt16Array",
	28: "UInt32Array",
	29: "UInt64Array",
	30: "FloatArray",
	31: "DoubleArray",
	32: "BooleanArray",
	33: "StringArray",
	34: "DateTimeArray",
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
