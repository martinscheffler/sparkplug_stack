package main

import (
	"bytes"
	"context"
	"github.com/jackc/pgx/v4"
	"html/template"
	"log"
)

var pgCon *pgx.Conn
var pgCtx = context.Background()
var sqlTemplate *template.Template = nil

func connectDB(postgresUrl string) error {
	log.Printf("Connecting to PostgreSQL on URL %v.\n", postgresUrl)
	pgConfig, err := pgx.ParseConfig(postgresUrl)
	if err != nil {
		log.Fatal("error parsing postgres config: ", err)
		return err
	}

	pgCon, err = pgx.ConnectConfig(pgCtx, pgConfig)
	if err != nil {
		log.Fatal("unable to connect to database: ", err)
		return err
	}
	log.Println("Connected to TimescaleDB.")
	return err
}

func disconnectDB() error {
	return pgCon.Close(pgCtx)
}

func storeSparkplugMessageToDB(sparkplugMessage *SparkplugMessage) error {

	var buffer bytes.Buffer
	err := sqlTemplate.Execute(&buffer, sparkplugMessage)
	if err != nil {
		return err
	}

	sql := buffer.String()
	_, err = pgCon.Exec(pgCtx, sql)
	return err
}

func loadSparkplugSqlTemplateFromFile(filename string) error {
	// load template from file
	tpl, err := template.New(filename).ParseFiles(filename)
	if err != nil {
		return err
	}
	sqlTemplate = tpl
	return nil
}

func getDevicesAndNodes() ([]Device, error) {
	sql := "select * from fetch_device_and_node_info()"
	rows, err := pgCon.Query(pgCtx, sql)
	if err != nil {
		return nil, err
	}

	var devices []Device
	for rows.Next() {
		var device Device
		if err := rows.Scan(&device.GroupId, &device.EdgeNodeID, &device.DeviceID, &device.LastBirth, &device.LastDeath); err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, nil
}
