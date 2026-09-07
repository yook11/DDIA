#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

case "${1:-}" in
    up)
        compose up -d --wait --wait-timeout 150
        ;;
    down)
        # Deliberately no --volumes. Only this Compose project's containers/network.
        compose down
        ;;
    migrate)
        # DDL goes only to the primary; PostgreSQL replicates it to the standbys.
        query primary admin -f /opt/ddia/migrate.sql
        ;;
    status)
        for node in primary replica1 replica2; do
            printf '\n--- %s ---\n' "$node"
            query "$node" admin -P format=aligned -P tuples_only=off -f "$DDIA_SQL/status.sql"
        done
        ;;
    psql)
        node=${2:-${NODE:-primary}}
        role=${3:-${ROLE:-app}}
        validate_node "$node"
        validate_role "$role"
        tty_args=(-T)
        if [[ -t 0 && -t 1 ]]; then tty_args=(--interactive); fi
        compose exec "${tty_args[@]}" -e PGCONNECT_TIMEOUT=3 "$node" bash -c '
            if [[ "$1" == admin ]]; then
                export PGPASSWORD="$POSTGRES_PASSWORD"; login="$POSTGRES_USER"
            else
                export PGPASSWORD="$DDIA_APP_PASSWORD"; login="$DDIA_APP_USER"
            fi
            exec psql -X -w -h 127.0.0.1 -U "$login" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1
        ' -- "$role"
        ;;
    pause)
        node=${2:-${NODE:-primary}}
        pause_replica "$node"
        printf '%s replay is paused. Resume soon: make db-resume NODE=%s\n' "$node" "$node"
        ;;
    resume)
        node=${2:-${NODE:-primary}}
        resume_replica "$node"
        printf '%s replay is running.\n' "$node"
        ;;
    *) fail 'Usage: db.sh up|down|migrate|status|psql [NODE] [ROLE]|pause NODE|resume NODE' ;;
esac
