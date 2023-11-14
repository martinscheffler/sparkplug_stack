docker cp ./perf_query.sql timescaledb:/tmp/perf_query.sql
docker cp ./perftest.sh timescaledb:/tmp/perftest.sh
docker exec timescaledb /bin/bash /tmp/perftest.sh
docker cp timescaledb:/tmp/results.txt .