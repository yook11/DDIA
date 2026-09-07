#!/usr/bin/env bash
# Shared by host-side commands. No Go build and no host psql are required.
DDIA_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
DDIA_SQL=/opt/ddia-lab/sql

compose() {
    docker compose --project-name ddia-replication-lag \
        --file "$DDIA_ROOT/infra/postgres/compose.yaml" "$@"
}

fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

validate_node() {
    case "$1" in primary|replica1|replica2) ;; *) fail "Unknown NODE: $1 (use primary, replica1 or replica2)" ;; esac
}

validate_replica() {
    case "$1" in replica1|replica2) ;; *) fail 'Replay control is allowed only for replica1 or replica2.' ;; esac
}

validate_role() {
    case "$1" in app|admin) ;; *) fail 'ROLE must be app or admin.' ;; esac
}

query() {
    local node=$1 role=$2
    shift 2
    validate_node "$node"
    validate_role "$role"
    compose exec -T -e PGCONNECT_TIMEOUT=3 \
        -e 'PGOPTIONS=-c statement_timeout=5000 -c lock_timeout=3000' "$node" \
        bash -c '
            role=$1; shift
            if [[ "$role" == admin ]]; then
                export PGPASSWORD="$POSTGRES_PASSWORD"
                login="$POSTGRES_USER"
            else
                export PGPASSWORD="$DDIA_APP_PASSWORD"
                login="$DDIA_APP_USER"
            fi
            exec psql -X -w -h 127.0.0.1 -U "$login" -d "$POSTGRES_DB" \
                -qAt -v ON_ERROR_STOP=1 "$@"
        ' -- "$role" "$@"
}

# SQL/connection errors fail immediately. Only a successful but not-yet-matching
# result is retried, and only until the deadline.
wait_for_sql() {
    local expected=$1 node=$2 role=$3 actual deadline=$((SECONDS + 30))
    shift 3
    while true; do
        actual=$(query "$node" "$role" "$@") || return 1
        [[ "$actual" == "$expected" ]] && return 0
        if (( SECONDS >= deadline )); then
            printf 'Timed out: %s expected [%s], got [%s]\n' "$node" "$expected" "$actual" >&2
            return 1
        fi
        sleep 0.2
    done
}

pause_replica() {
    validate_replica "$1"
    query "$1" admin -f "$DDIA_SQL/pause.sql" >/dev/null || return 1
    wait_for_sql paused "$1" admin -f "$DDIA_SQL/pause-state.sql"
}

resume_replica() {
    validate_replica "$1"
    query "$1" admin -f "$DDIA_SQL/resume.sql" >/dev/null || return 1
    wait_for_sql 'not paused' "$1" admin -f "$DDIA_SQL/pause-state.sql"
}
