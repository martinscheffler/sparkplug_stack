package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v4"
	"html/template"
	"log"
	"time"
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

	query := buffer.String()
	_, err = pgCon.Exec(pgCtx, query)
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

type NodeListEntry struct {
	GroupId    string
	EdgeNodeId string
	DeviceId   string
	IsOnline   bool
}

func getNodes() ([]NodeListEntry, error) {
	query := `
		SELECT
            b.group_id,
            b.edge_node_id,
            b.device_id,
            b.timestamp as birth_time,
            d.received_at as death_time
        FROM birth AS b
        LEFT JOIN public.death AS d
           ON
           d.edge_node_id=b.edge_node_id AND
           d.group_id=b.group_id AND
           d.device_id=b.device_id
        ORDER BY device_id, edge_node_id;
	`
	rows, err := pgCon.Query(pgCtx, query)
	if err != nil {
		return nil, err
	}

	var nodes []NodeListEntry
	for rows.Next() {
		var node NodeListEntry
		var lastBirth time.Time
		var lastDeath *time.Time
		if err := rows.Scan(&node.GroupId, &node.EdgeNodeId, &node.DeviceId, &lastBirth, &lastDeath); err != nil {
			return nil, err
		}
		node.IsOnline = false
		if lastDeath == nil || lastDeath.Before(lastBirth) {
			node.IsOnline = true
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

type NodeInfo struct {
	GroupId    string
	EdgeNodeId string
	DeviceId   string
	LastBirth  time.Time
	LastDeath  *time.Time // nullable
}

func getNodeInfo(groupId string, nodeId string, deviceId string) (*NodeInfo, error) {
	query := `
		SELECT
		b.group_id,
		b.edge_node_id,
		b.device_id,
		b.timestamp as birth_time,
		d.received_at as death_time
		FROM birth AS b
		    LEFT JOIN public.death AS d
		        ON
		            d.edge_node_id=b.edge_node_id AND
		            d.group_id=b.group_id AND
		            d.device_id=b.device_id
		WHERE b.group_id=$1
		AND b.edge_node_id=$2
		AND b.device_id=$3
		ORDER BY group_id, device_id, edge_node_id
		`
	row := pgCon.QueryRow(pgCtx, query, groupId, nodeId, deviceId)
	var nodeInfo NodeInfo
	err := row.Scan(&nodeInfo.GroupId, &nodeInfo.EdgeNodeId, &nodeInfo.DeviceId, &nodeInfo.LastBirth, &nodeInfo.LastDeath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("no rows were returned")
		}
		return nil, err
	}

	return &nodeInfo, nil
}
