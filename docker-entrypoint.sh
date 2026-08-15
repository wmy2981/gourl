#!/bin/sh
# Single-container entrypoint: starts the embedded Redis on 127.0.0.1:6379
# unless REDIS_ADDR points at an external instance, then runs gourl.
set -e

if [ -z "$REDIS_ADDR" ]; then
  echo "starting embedded redis on 127.0.0.1:6379"
  redis-server --daemonize yes \
    --bind 127.0.0.1 --port 6379 \
    --dir /app/data --dbfilename redis.rdb \
    --appendonly yes --appendfilename redis.aof
  export REDIS_ADDR=127.0.0.1:6379
else
  echo "using external redis at $REDIS_ADDR"
fi

exec gourl
