#!/usr/bin/env bash
set -Eeuo pipefail

fail() { printf '%s\n' "${DDIA_NODE}: $*" >&2; exit 1; }
case "$DDIA_NODE" in replica1|replica2) ;; *) fail 'Unknown replica name.' ;; esac

if [[ "$(id -u)" == 0 ]]; then
    mkdir -p "$PGDATA" /var/run/postgresql
    chown postgres:postgres "$PGDATA" /var/run/postgresql
    chmod 700 "$PGDATA"
    exec gosu postgres bash "$0" "$@"
fi

# Recreate the container-local password file on every start; do not put the password
# into primary_conninfo or depend on a password file inside the copied PGDATA.
export PGPASSFILE=/tmp/ddia-replication.pgpass
umask 077
printf 'primary:5432:replication:ddia_replicator:%s\n' "$DDIA_REPLICATION_PASSWORD" > "$PGPASSFILE"
unset PGPASSWORD

if [[ -s "$PGDATA/PG_VERSION" ]]; then
    [[ "$(< "$PGDATA/PG_VERSION")" == 18 ]] || fail 'Unexpected PostgreSQL version.'
    [[ -f "$PGDATA/.ddia-replica-ready" && -f "$PGDATA/standby.signal" && -s "$PGDATA/global/pg_control" ]] ||
        fail 'Incomplete or wrong-role data directory; existing files are preserved. Inspect logs before resetting.'
else
    [[ -z "$(find "$PGDATA" -mindepth 1 -maxdepth 1 -print -quit)" ]] ||
        fail 'Nonempty data directory; refusing to overwrite it.'
    # --no-clean deliberately preserves evidence of an interrupted/failed backup.
    timeout 90s pg_basebackup \
        --dbname="host=primary port=5432 user=ddia_replicator application_name=$DDIA_NODE passfile=$PGPASSFILE connect_timeout=5" \
        --pgdata="$PGDATA" --wal-method=stream --write-recovery-conf \
        --slot="$DDIA_NODE" --checkpoint=fast --no-clean ||
        fail 'Initial backup failed. No automatic deletion or reinitialization will be attempted.'
    [[ -f "$PGDATA/standby.signal" && -s "$PGDATA/global/pg_control" ]] || fail 'Incomplete backup.'
    touch "$PGDATA/.ddia-replica-ready"
fi

exec "$@"
