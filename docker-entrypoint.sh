#!/bin/sh
# Single-container entrypoint: starts the embedded Redis on 127.0.0.1:6379
# unless REDIS_ADDR points at an external instance, then runs gourl.
#
# The entrypoint runs as root on purpose: freshly created bind mounts (a first
# deployment with no ./data directory yet) inherit the host's root ownership,
# which the gourl user could never write to. mkdir + chown the mount points
# first (mkdir covers `gourl reset --all`, which deletes both directories),
# then drop privileges with su-exec for both Redis and gourl.
set -e

mkdir -p /app/data /app/config
chown -R gourl:gourl /app/data /app/config

if [ -z "$REDIS_ADDR" ]; then
  echo "starting embedded redis on 127.0.0.1:6379"
  su-exec gourl redis-server --daemonize yes \
    --bind 127.0.0.1 --port 6379 \
    --dir /app/data --dbfilename redis.rdb \
    --appendonly yes --appendfilename redis.aof
  export REDIS_ADDR=127.0.0.1:6379
else
  echo "using external redis at $REDIS_ADDR"
fi

exec su-exec gourl gourl
