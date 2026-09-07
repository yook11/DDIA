#!/usr/bin/env bash
set -Eeuo pipefail

fail() { printf '%s\n' "primary: $*" >&2; exit 1; }

if [[ -s "$PGDATA/PG_VERSION" ]]; then
    [[ "$(< "$PGDATA/PG_VERSION")" == 18 ]] || fail 'Unexpected PostgreSQL version; refusing to initialize.'
    [[ -f "$PGDATA/.ddia-primary-ready" && -s "$PGDATA/global/pg_control" ]] ||
        fail 'Incomplete initialization. Existing data is preserved; inspect logs and the documented reset procedure.'
    [[ ! -f "$PGDATA/standby.signal" ]] || fail 'This volume belongs to a standby; refusing to start as primary.'
elif [[ -d "$PGDATA" && -n "$(find "$PGDATA" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
    fail 'Nonempty data directory without PG_VERSION; refusing to overwrite it.'
fi

# The official initializer also connects over the password-protected local socket.
export PGPASSWORD="$POSTGRES_PASSWORD"
exec /usr/local/bin/docker-entrypoint.sh "$@"
