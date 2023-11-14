
CREATE EXTENSION IF NOT EXISTS timescaledb;


CREATE TYPE message_type AS ENUM (
    'NBIRTH',
    'NDEATH',
    'DBIRTH',
    'DDEATH',
    'NDATA',
    'DDATA',
    'NCMD',
    'DCMD',
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

create table public.birth_data
(
    group_id TEXT NOT NULL,
    edge_node_id TEXT NOT NULL,
    device_id TEXT NULL,
    timestamp TIMESTAMPTZ NULL,
    metrics metric_type[],
    seq BIGINT NULL,
    prop_hardware_make TEXT,
    prop_hardware_model TEXT,
    prop_fw TEXT,
    prop_os TEXT,
    prop_os_version TEXT,
    received_at TIMESTAMPTZ NOT NULL,
    death_received_AT TIMESTAMPTZ,
    CONSTRAINT constraint_name UNIQUE (group_id, edge_node_id, device_id)
);

CREATE OR REPLACE PROCEDURE insert_sparkplug_payload(
    p_group_id TEXT,
    p_message_type message_type,
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
    current_ts timestamp := now();
    metrics_length INT;
    p_hardware_make TEXT;
    p_hardware_model TEXT;
    p_fw TEXT;
    p_os TEXT;
    p_os_version TEXT;
BEGIN
    -- store whole payload plus topic to data table --
    INSERT INTO data
    (received_at, group_id, message_type, edge_node_id, device_id, timestamp, seq, uuid, body, metrics)
    VALUES
        (current_ts, p_group_id, p_message_type, p_edge_node_id, p_device_id, p_timestamp, p_seq, p_uuid, p_body, p_metrics);

    -- store birth messages to birth_data table --
    IF p_message_type = 'DBIRTH'::message_type OR p_message_type = 'NBIRTH'::message_type THEN

        metrics_length := array_length(p_metrics, 1); -- Get the length of the array
        IF metrics_length != 0 THEN
            FOR i IN 1..metrics_length LOOP
                -- Compare the array value with property names
                IF p_metrics[i].name = 'Properties/Hardware Make' THEN
                    p_hardware_make := p_metrics[i].value_string;
                ELSEIF p_metrics[i].name = 'Properties/Hardware Model' THEN
                    p_hardware_model := p_metrics[i].value_string;
                ELSEIF p_metrics[i].name = 'Properties/FW' THEN
                    p_fw := p_metrics[i].value_string;
                ELSEIF p_metrics[i].name = 'Properties/OS' THEN
                    p_os := p_metrics[i].value_string;
                ELSEIF p_metrics[i].name = 'Properties/OS Version' THEN
                    p_os_version := p_metrics[i].value_string;
                END IF;
            END LOOP;
        END IF;
        -- Insert if no entry exists yet --
        INSERT INTO birth_data (
                group_id, edge_node_id, device_id, timestamp, metrics,
                                seq, prop_hardware_make, prop_hardware_model,
                                prop_fw, prop_os, prop_os_version,
                                received_at, death_received_at
        )
        VALUES (
                p_group_id, p_edge_node_id, p_device_id, p_timestamp, p_metrics,
                p_seq, p_hardware_make, p_hardware_model,
                    p_fw, p_os, p_os_version, current_ts, null)
        ON CONFLICT (group_id, edge_node_id, device_id) DO UPDATE
            -- Update if it already exists --
            SET timestamp=p_timestamp,
                metrics=p_metrics,
                seq=p_seq,
                prop_hardware_make=p_hardware_make,
                prop_hardware_model=p_hardware_model,
                prop_fw=p_fw,
                prop_os=p_os,
                prop_os_version=p_os_version,
                received_at=current_ts,
                death_received_at=null;

    -- For death messages, update death_received value in birth_data table --
    ELSIF p_message_type = 'DDEATH'::message_type OR p_message_type = 'NDEATH'::message_type THEN
        UPDATE birth_data
        SET death_received_at=current_ts
        WHERE EXISTS (
            SELECT 1 FROM birth_data
                     WHERE birth_data.group_id=p_group_id
                     AND birth_data.edge_node_id=p_edge_node_id
                     AND birth_data.device_id=p_device_id
        );
    END IF;
END;
$$;


CREATE OR REPLACE FUNCTION fetch_float_metrics_by_name(p_group_id TEXT,
                                                       p_message_type message_type,
                                                       p_edge_node_id TEXT,
                                                       p_device_id TEXT,
                                                       p_metric_name TEXT,
                                                       p_limit INT
)
    RETURNS TABLE (
                      "time" TIMESTAMPTZ,
                      "value" REAL) AS
$$
BEGIN
RETURN QUERY
SELECT
    m.timestamp as "time",
    m.value_float as "value"
FROM data as d
         CROSS JOIN LATERAL unnest(d.metrics) AS m
WHERE d.group_id=p_group_id
  AND d.message_type=p_message_type
  AND d.edge_node_id=p_edge_node_id
  AND d.device_id=p_device_id
  AND m.name=p_metric_name
  ORDER BY m.timestamp DESC LIMIT p_limit;
END;
$$
LANGUAGE plpgsql;


CREATE OR REPLACE FUNCTION fetch_double_metrics_by_name(p_group_id TEXT,
                                                       p_message_type message_type,
                                                       p_edge_node_id TEXT,
                                                       p_device_id TEXT,
                                                       p_metric_name TEXT,
                                                       p_limit INT
)
    RETURNS TABLE (
                      "time" TIMESTAMPTZ,
                      "value" DOUBLE PRECISION) AS
$$
BEGIN
RETURN QUERY
SELECT
    m.timestamp as "time",
    m.value_double as "value"
FROM data as d
         CROSS JOIN LATERAL unnest(d.metrics) AS m
WHERE d.group_id=p_group_id
  AND d.message_type=p_message_type
  AND d.edge_node_id=p_edge_node_id
  AND d.device_id=p_device_id
  AND m.name=p_metric_name
ORDER BY m.timestamp DESC LIMIT p_limit;
END;
$$
LANGUAGE plpgsql;
