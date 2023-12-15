
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS tablefunc;

CREATE TYPE message_type AS ENUM (
    'BIRTH',
    'DEATH',
    'DATA',
    'CMD',
    'STATE'
    );


CREATE TYPE metadata_type AS (
    is_multi_part BOOLEAN,
    content_type TEXT,
    size BIGINT,
    seq BIGINT,
    file_name TEXT,
    file_type TEXT,
    md5 TEXT,
    description TEXT
    );

CREATE TYPE propertyvalue_type AS (
    "type" INT,
    is_null BOOLEAN,
    int_value INT,
    long_value BIGINT,
    float_value FLOAT,
    double_value DOUBLE PRECISION,
    boolean_value BOOLEAN,
    string_value TEXT
    --- TODO figure out how to handle circular reference
    --- propertyset_value propertyset_type,
    --- propertyset_list propertyset_type[]

    );

CREATE TYPE propertyset_type AS (
    keys TEXT[],
    values propertyvalue_type[]
    );


CREATE TYPE metric_type AS (
    "name" TEXT,
    "alias" BIGINT,
    "timestamp" TIMESTAMPTZ,
    "datatype" INTEGER,
    is_historical BOOLEAN,
    is_transient BOOLEAN,
    is_null BOOLEAN,
    metadata metadata_type,
    properties propertyset_type,
    value_string TEXT,
    value_bool BOOLEAN,
    value_int INTEGER,
    value_uint64 BIGINT,
    value_double DOUBLE PRECISION,
    value_float REAL
    );


create table public.data
(
    group_id TEXT NOT NULL,
    message_type message_type NOT NULL,
    edge_node_id TEXT NOT NULL,
    device_id TEXT NULL,
    timestamp TIMESTAMPTZ NULL,
    metrics metric_type[],
    seq BIGINT NULL,
    uuid TEXT NULL,
    body bytea NULL,
    received_at TIMESTAMPTZ NOT NULL
    );

SELECT create_hypertable('data', 'received_at');


create table public.birth
(
    group_id TEXT NOT NULL,
    edge_node_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    timestamp TIMESTAMPTZ NULL,
    metrics metric_type[],
    received_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT unique_birth_per_node UNIQUE (group_id, edge_node_id, device_id)

);


create table public.metrics_info
(
    group_id TEXT NOT NULL,
    edge_node_id TEXT NOT NULL,
    device_id TEXT NULL,
    "name" TEXT,
    alias INT,
    properties jsonb,
    CONSTRAINT unique_alias_per_node UNIQUE (group_id, edge_node_id, device_id, name)
);


create table public.death
(
    group_id TEXT NOT NULL,
    edge_node_id TEXT NOT NULL,
    device_id TEXT NULL,
    timestamp TIMESTAMPTZ NULL,
    received_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT unique_death_per_node UNIQUE (group_id, edge_node_id, device_id)
);


CREATE OR REPLACE PROCEDURE insert_sparkplug_payload(
    p_group_id TEXT,
    p_message_type TEXT,
    p_edge_node_id TEXT,
    p_device_id TEXT,
    p_timestamp TIMESTAMPTZ,
    p_seq BIGINT,
    p_uuid TEXT,
    p_body bytea,
    p_metrics metric_type[]
)
    LANGUAGE plpgsql
AS $$
DECLARE
    msg_type message_type;
    received_ts TIMESTAMPTZ := now();
    metric metric_type;
BEGIN
    msg_type := CASE
            WHEN p_message_type='DDATA' or p_message_type='NDATA' THEN 'DATA'::message_type
            WHEN p_message_type='DBIRTH' or p_message_type='NBIRTH' THEN 'BIRTH'::message_type
            WHEN p_message_type='DDEATH' or p_message_type='NDEATH' THEN 'DEATH'::message_type
            WHEN p_message_type='DCMD' or p_message_type='NCMD' THEN 'CMD'::message_type
            WHEN p_message_type='STATE' THEN 'STATE'::message_type
    END;
        -- store whole payload plus topic to data table --
    INSERT INTO data
        (message_type, received_at, group_id, edge_node_id, device_id, timestamp, seq, uuid, body, metrics)
    VALUES
        (msg_type, received_ts, p_group_id, p_edge_node_id, p_device_id, p_timestamp, p_seq, p_uuid, p_body, p_metrics);

    IF msg_type='BIRTH' THEN

        UPDATE birth
        SET timestamp=p_timestamp, received_at=received_ts, metrics=p_metrics
        WHERE group_id=p_group_id
        AND edge_node_id=p_edge_node_id
        AND device_id=p_device_id;

        -- if no rows were updated, insert the new data
        IF NOT FOUND THEN
                    INSERT INTO birth
                    (group_id, edge_node_id, device_id, "timestamp", metrics, received_at)
                    VALUES
                        (
                            p_group_id,
                            p_edge_node_id,
                            p_device_id,
                            p_timestamp,
                            p_metrics,
                            received_ts
                        );
        END IF;

        FOREACH metric IN ARRAY p_metrics LOOP
             -- RAISE WARNING 'Metric: % Alias: %', metric.name, metric.alias;--
            IF metric.alias IS NOT NULL THEN
                INSERT INTO metrics_info
                    (group_id, edge_node_id, device_id, "name", alias)
                VALUES
                    (p_group_id, p_edge_node_id, p_device_id, metric.name, metric.alias)
                ON CONFLICT(group_id, edge_node_id, device_id, "name")
                    DO UPDATE SET alias=metric.alias;
            END IF;
        END LOOP;
    ELSEIF msg_type='DEATH' THEN
        UPDATE death
        SET timestamp=p_timestamp, received_at=received_ts
        WHERE group_id=p_group_id
          AND edge_node_id=p_edge_node_id
          AND device_id=p_device_id;

        -- if no rows were updated, insert the new data
        IF NOT FOUND THEN
            INSERT INTO death
            (group_id, edge_node_id, device_id, "timestamp",received_at)
            VALUES
                (
                    p_group_id,
                    p_edge_node_id,
                    p_device_id,
                    p_timestamp,
                    received_ts
                );
        END IF;
    END IF;
END
$$;


CREATE OR REPLACE FUNCTION fetch_metrics(p_group_id TEXT, p_device_name TEXT, p_time_from TIMESTAMPTZ, p_time_to TIMESTAMPTZ, p_limit INT)
    RETURNS TABLE ("time" TIMESTAMPTZ, "metric" TEXT, "value" DOUBLE PRECISION) AS
$$
DECLARE
edge_part TEXT := SPLIT_PART(p_device_name, '.', 1);
    device_part TEXT := SPLIT_PART(p_device_name, '.', 2);
BEGIN
RETURN QUERY

SELECT
    d.received_at as time,
            COALESCE(m.name, a.name) as name,
            COALESCE(
                    m.value_double,
                    CAST(m.value_int AS DOUBLE PRECISION),
                    CAST(m.value_uint64 AS DOUBLE PRECISION),
                    CAST(m.value_float AS DOUBLE PRECISION)
        ) AS value
FROM data as d
    CROSS JOIN LATERAL unnest(d.metrics) AS m
    LEFT JOIN metrics_info a ON m.alias = a.alias
WHERE d.group_id= p_group_id
  AND d.message_type in ('BIRTH', 'DATA')
  AND m.datatype NOT in(12, 13, 14) -- ignore text and date metrics --
  AND d.edge_node_id=edge_part
  AND d.device_id=device_part
  AND d.received_at > p_time_from
  AND d.received_at < p_time_to
ORDER BY d.received_at ASC LIMIT p_limit;
END;
$$
LANGUAGE plpgsql;


-- Returns a single column with a list of all devices and nodes.
-- For nodes, the node_id is included.
-- Devices are included in format [node id].[device id].
CREATE OR REPLACE FUNCTION fetch_all_devices_and_nodes(p_group_id TEXT)
    RETURNS TABLE (device TEXT) AS
$$
BEGIN
    RETURN QUERY
        SELECT
            CASE
                WHEN length(device_id) = 0 THEN edge_node_id
                ELSE CONCAT(edge_node_id, '.', device_id)
                END AS device
        FROM birth
        WHERE group_id=p_group_id
        ORDER BY device_id, edge_node_id ASC;

END;
$$
    LANGUAGE plpgsql;

-- Returns a single column with a list of all devices and nodes.
-- For nodes, the node_id is included.
-- Devices are included in format [node id].[device id].
CREATE OR REPLACE FUNCTION fetch_device_and_node_info()
    RETURNS TABLE (
        group_id TEXT,
        edge_node_id TEXT,
        device_id TEXT,
        birth_time TIMESTAMPTZ,
        death_time TIMESTAMPTZ
    ) AS
$$
BEGIN
    RETURN QUERY
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
        ORDER BY device_id, edge_node_id ASC;
END;
$$
    LANGUAGE plpgsql;


CREATE OR REPLACE FUNCTION propertyset_to_json(properties propertyset_type)
    RETURNS TABLE(json jsonb)
AS $$
DECLARE
    key_index INTEGER;
BEGIN
    -- initialize an empty JSON object
    json := '{}'::jsonb;
    IF properties IS NOT NULL THEN

        -- iterate over the keys and values in the propertyset_type, and add each key-value pair to the JSON object
        FOR key_index IN 1..array_length(properties.keys, 1) LOOP
                json := json || jsonb_build_object(
                        properties.keys[key_index],
                        COALESCE(to_jsonb(properties.values[key_index].string_value),
                                 to_jsonb(properties.values[key_index].int_value),
                                 to_jsonb(properties.values[key_index].long_value),
                                 to_jsonb(properties.values[key_index].float_value),
                                 to_jsonb(properties.values[key_index].double_value),
                                 to_jsonb(properties.values[key_index].boolean_value)
                        )
                                );
            END LOOP;
    END IF;

    -- return the JSON object in a table with a single row and column
    RETURN QUERY SELECT json;
END;
$$ LANGUAGE plpgsql;


CREATE OR REPLACE FUNCTION fetch_grafana_config(p_group_id TEXT, p_device_name TEXT)
    RETURNS TABLE ("name" TEXT, "color" TEXT, "unit" TEXT, js jsonb) AS
$$
DECLARE
    edge_part TEXT := SPLIT_PART(p_device_name, '.', 1);
    device_part TEXT := SPLIT_PART(p_device_name, '.', 2);
BEGIN
    RETURN QUERY
        SELECT
            m.name,
            props_json->>'Grafana/Color' as color,
            props_json->>'Grafana/Unit' as unit,
            props_json
        FROM birth as b
                CROSS JOIN LATERAL unnest(b.metrics) AS m
                CROSS JOIN LATERAL propertyset_to_json(m.properties) as props_json
        WHERE b.group_id=p_group_id
          AND b.edge_node_id=edge_part
          AND b.device_id=device_part
          AND props_json <> '{}'::jsonb
        ORDER BY m.name ASC;
END;
$$
    LANGUAGE plpgsql;


