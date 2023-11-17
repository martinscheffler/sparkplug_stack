
CREATE EXTENSION IF NOT EXISTS timescaledb;


CREATE TYPE message_type AS ENUM (
    'BIRTH',
    'DEATH',
    'DATA',
    'CMD',
    'STATE'
    );


CREATE TYPE metadata_type AS (
    is_multi_part bool,
    content_type TEXT,
    size BIGINT,
    seq BIGINT,
    file_name TEXT,
    file_type TEXT,
    md5 TEXT,
    description TEXT
    );


CREATE TYPE propertyset_type AS (
    keys TEXT[],
    values jsonb[]
    );


CREATE TYPE metric_type AS (
    "name" TEXT,
    "alias" BIGINT,
    "timestamp" TIMESTAMPTZ,
    "datatype" INTEGER,
    is_historical bool,
    is_transient bool,
    is_null bool,
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


create table public.last_birth_msg
(
    group_id TEXT NOT NULL,
    edge_node_id TEXT NOT NULL,
    device_id TEXT NULL,
    timestamp TIMESTAMPTZ NULL,
    metrics metric_type[],
    received_at TIMESTAMPTZ NOT NULL
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

        UPDATE last_birth_msg
        SET timestamp=p_timestamp, metrics=p_metrics, received_at=received_ts
        WHERE group_id=p_group_id
          AND edge_node_id=p_edge_node_id
          AND device_id=p_device_id;

        -- if no rows were updated, insert the new data
        IF NOT FOUND THEN
                    INSERT INTO last_birth_msg
                    (group_id, edge_node_id, device_id, timestamp, metrics, received_at)
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

    END IF;
END
$$;

CREATE OR REPLACE FUNCTION fetch_double_metrics(p_group_id TEXT, p_device_name TEXT,p_time_from TIMESTAMPTZ, p_time_to TIMESTAMPTZ, p_limit INT)
    RETURNS TABLE (
                      "time" TIMESTAMPTZ,
                      "value" DOUBLE PRECISION) AS
$$
DECLARE
    edge_part TEXT := SPLIT_PART(p_device_name, '.', 1);
    device_part TEXT := SPLIT_PART(p_device_name, '.', 2);
BEGIN
RETURN QUERY
SELECT
    m.timestamp as "time",
    m.value_double as "value"
FROM data as d
         CROSS JOIN LATERAL unnest(d.metrics) AS m
WHERE d.group_id=p_group_id
  AND d.message_type='DATA'
  AND d.edge_node_id=edge_part
  AND d.device_id=device_part
  AND m.timestamp > p_time_from
  AND m.timestamp < p_time_to
ORDER BY m.timestamp DESC LIMIT p_limit;
END;
$$
LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fetch_all_devices_and_nodes(p_group_id TEXT)
    RETURNS TABLE (device TEXT) AS
$$
BEGIN
RETURN QUERY
SELECT
    CASE
        WHEN d.device_id IS NULL THEN d.edge_node_id
        ELSE CONCAT(d.edge_node_id, '.', d.device_id)
        END AS device
FROM data as d
WHERE d.group_id=p_group_id
  AND d.message_type ='BIRTH'
GROUP by d.edge_node_id, d.device_id
ORDER BY d.edge_node_id, d.device_id ASC;

END;
$$
LANGUAGE plpgsql;

