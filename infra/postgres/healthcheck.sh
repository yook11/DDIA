#!/usr/bin/env bash
set -Eeuo pipefail
export PGPASSWORD="$POSTGRES_PASSWORD" PGCONNECT_TIMEOUT=2
export PGOPTIONS='-c statement_timeout=2000'

case "$DDIA_NODE" in
    primary) expected=f ;;
    replica1|replica2) expected=t ;;
    *) exit 1 ;;
esac

# TCP avoids mistaking the socket-only initialization server for the ready primary.
actual=$(psql -X -w -h 127.0.0.1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At \
    -v ON_ERROR_STOP=1 -c "SELECT pg_is_in_recovery() WHERE to_regclass('lab.posts') IS NOT NULL")
[[ "$actual" == "$expected" ]]
