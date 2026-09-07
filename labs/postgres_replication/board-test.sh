#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../../infra/postgres/common.sh"

printf '1. Verify the primary schema and application-role constraints.\n'
query primary app -f "$DDIA_SQL/board-schema.sql"
query primary app -f "$DDIA_SQL/board-constraints.sql"

printf '2. Wait for schema replication and verify both replicas.\n'
required_lsn=$(query primary app -c 'SELECT pg_current_wal_lsn()')
for node in replica1 replica2; do
    wait_for_sql t "$node" app -v required_lsn="$required_lsn" -f "$DDIA_SQL/caught-up.sql"
    query "$node" app -f "$DDIA_SQL/board-schema.sql"
    if output=$(query "$node" app -v VERBOSITY=sqlstate \
        -c "BEGIN; INSERT INTO board.users (name) VALUES ('read-only probe'); ROLLBACK;" 2>&1); then
        fail "$node accepted a board INSERT"
    else
        [[ "$output" == *25006* ]] || fail "$node failed for a reason other than read-only: $output"
    fi
done
printf 'PASS: board tables, generated IDs, constraints, permissions, and schema replication. Fixture rows were rolled back.\n'
