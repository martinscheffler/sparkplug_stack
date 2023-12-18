package main

import (
	"bytes"
	"fmt"
	"github.com/jackc/pgtype"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"html/template"
	"log"
	"strings"
	"time"
)

var sqlTemplate *template.Template = nil
var db *sqlx.DB

func connectDB(postgresUrl string) error {
	log.Printf("Connecting to PostgreSQL on URL %v.\n", postgresUrl)
	var err error
	db, err = sqlx.Connect("pgx", postgresUrl)
	if err == nil {
		log.Println("Connected to TimescaleDB.")
	}
	return err
}

func disconnectDB() error {
	return db.Close()
}

func storeSparkplugMessageToDB(sparkplugMessage *SparkplugMessage) error {

	var buffer bytes.Buffer
	err := sqlTemplate.Execute(&buffer, sparkplugMessage)
	if err != nil {
		return err
	}

	query := buffer.String()
	_, err = db.Exec(query)
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
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	var nodes []NodeListEntry
	for rows.Next() {
		var node NodeListEntry
		var lastBirth pgtype.Timestamp
		var lastDeath *pgtype.Timestamp
		if err := rows.Scan(&node.GroupId, &node.EdgeNodeId, &node.DeviceId, &lastBirth, &lastDeath); err != nil {
			return nil, err
		}
		node.IsOnline = false
		if lastDeath == nil || lastDeath.Time.Before(lastBirth.Time) {
			node.IsOnline = true
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

type PropertyValue struct {
	Type         int32
	IsNull       bool
	IntValue     *int32
	LongValue    *int64
	FloatValue   *float32
	DoubleValue  *float64
	BooleanValue *bool
	StringValue  *string
}

type PropertySet struct {
	keys   []string
	values []PropertyValue
}

type MetaData struct {
	IsMultiPart bool
	ContentType string
	Size        int64
	Seq         int64
	FileName    string
	FileType    string
	Md5         string
	Description string
}

type Metric struct {
	Name         string
	Alias        int64
	Timestamp    time.Time
	DataType     int32
	IsHistorical bool
	IsTransient  bool
	IsNull       bool
	Metadata     *MetaData
	Properties   *PropertySet
	ValueString  *string
	ValueBool    *bool
	ValueInt     *int32
	ValueUint64  *uint64
	ValueDouble  *float64
	ValueFloat   *float32
}

func (m *Metric) Scan(src interface{}) error {
	source, ok := src.([]uint8)
	if !ok {
		return fmt.Errorf("type assertion to string failed")
	}
	sourceStr := string(source)
	sourceStr = strings.Trim(sourceStr, "()")
	parts := strings.Split(sourceStr, ",")
	m.Name = strings.Trim(parts[0], "\"")
	var err error
	if err == nil && parts[1] != "" {
		_, err = fmt.Sscanf(parts[1], "%d", &m.Alias)

	}
	if err == nil && parts[2] != "" {

		m.Timestamp, err = time.Parse("\"2006-01-02 15:04:05.000-07\"", parts[2])
	}

	if err == nil && parts[3] != "" {
		_, err = fmt.Sscanf(parts[3], "%d", &m.DataType)
	}
	if err == nil && parts[4] != "" {
		_, err = fmt.Sscanf(parts[4], "%t", &m.IsHistorical)
	}

	if err == nil && parts[5] != "" {
		_, err = fmt.Sscanf(parts[5], "%t", &m.IsTransient)
	}
	if err == nil && parts[6] != "" {
		_, err = fmt.Sscanf(parts[6], "%t", &m.IsNull)
	}
	if err == nil && parts[9] != "" {
		m.ValueString = &parts[9]
	}
	if err == nil && parts[10] != "" {
		var boolVal bool
		_, err = fmt.Sscanf(parts[10], "%t", &boolVal)
		m.ValueBool = &boolVal
	}
	if err == nil && parts[11] != "" {
		var intVal int32
		_, err = fmt.Sscanf(parts[11], "%d", &intVal)
		m.ValueInt = &intVal
	}
	if err == nil && parts[12] != "" {
		var uint64Val uint64
		_, err = fmt.Sscanf(parts[12], "%d", &uint64Val)
		m.ValueUint64 = &uint64Val
	}
	if err == nil && parts[13] != "" {
		var doubleVal float64
		_, err = fmt.Sscanf(parts[13], "%F", &doubleVal)
		m.ValueDouble = &doubleVal
	}
	if err == nil && parts[14] != "" {
		var floatVal float32
		_, err = fmt.Sscanf(parts[14], "%f", &floatVal)
		m.ValueFloat = &floatVal
	}

	return err
}

type NodeInfo struct {
	GroupId    string            `db:"group_id"`
	EdgeNodeId string            `db:"node_id"`
	DeviceId   string            `db:"device_id"`
	LastBirth  pgtype.Timestamp  `db:"last_birth"`
	LastDeath  *pgtype.Timestamp `db:"last_death"` // nullable
	Metrics    []Metric          `db:"metrics"`
}

func getNodeInfo(groupId string, nodeId string, deviceId string) (*NodeInfo, error) {
	query := `
		SELECT
		b.timestamp as last_birth,
		d.received_at as last_death,
		b.metrics as metrics
		FROM birth AS b
		    LEFT JOIN public.death AS d
		        ON
		            d.edge_node_id=b.edge_node_id AND
		            d.group_id=b.group_id AND
		            d.device_id=b.device_id
		WHERE b.group_id=$1
		AND b.edge_node_id=$2
		AND b.device_id=$3
		ORDER BY b.group_id, b.device_id, b.edge_node_id
		`
	var lastBirth pgtype.Timestamp
	var lastDeath *pgtype.Timestamp
	row := db.QueryRowx(query, groupId, nodeId, deviceId)
	var metrics []Metric
	err := row.Scan(&lastBirth, &lastDeath, pq.Array(&metrics))
	if err != nil {
		return nil, err
	}
	nodeInfo := NodeInfo{GroupId: groupId, EdgeNodeId: nodeId, DeviceId: deviceId, LastBirth: lastBirth, LastDeath: lastDeath, Metrics: metrics}
	return &nodeInfo, nil

}
