#!/bin/bash

/usr/local/bin/pgbench -r -U postgres -d gosp3 -c 10 -T 20 -j 10 -f /tmp/perf_query.sql > /tmp/results.txt 2>&1