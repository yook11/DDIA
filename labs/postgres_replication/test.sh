#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../../infra/postgres/common.sh"

assert_sql() {
    local expected=$1 node=$2 role=$3 actual
    shift 3
    actual=$(query "$node" "$role" "$@") || return 1
    [[ "$actual" == "$expected" ]] || fail "$node: expected [$expected], got [$actual] ($*)"
}

pause_owned=0
cleanup() {
    local status=$?
    trap - EXIT INT TERM HUP
    if (( pause_owned )); then
        printf 'Cleanup: resuming replica1 replay.\n' >&2
        if ! resume_replica replica1; then
            printf 'WARNING: automatic resume failed. Run: make db-resume NODE=replica1\n' >&2
            status=1
        fi
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

printf '1. Verify roles, read-only replicas, and asynchronous replication.\n'
assert_sql 'primary|off|on|' primary app -f "$DDIA_SQL/role.sql"
for node in replica1 replica2; do
    assert_sql 'replica|on|on|' "$node" app -f "$DDIA_SQL/role.sql"
    # Existing user pauses are not modified. Resume them explicitly before testing.
    assert_sql 'not paused' "$node" admin -f "$DDIA_SQL/pause-state.sql"
    if output=$(query "$node" app -v VERBOSITY=sqlstate -f "$DDIA_SQL/read-only.sql" 2>&1); then
        fail "$node accepted an INSERT"
    else
        [[ "$output" == *25006* ]] || fail "$node failed for a reason other than read-only: $output"
    fi
done

printf '2. Write a baseline and wait for both replicas.\n'
baseline=$(query primary app -v body='db-test baseline' -f "$DDIA_SQL/write.sql")
baseline_id=${baseline%%|*}
baseline_lsn=${baseline#*|}
for node in replica1 replica2; do
    wait_for_sql t "$node" app -v required_lsn="$baseline_lsn" -f "$DDIA_SQL/caught-up.sql"
    assert_sql t "$node" app -v post_id="$baseline_id" -f "$DDIA_SQL/visible.sql"
done

printf '3. Pause replica1 and confirm it is actually paused.\n'
# Set ownership before the request so even an error during confirmation triggers cleanup.
pause_owned=1
pause_replica replica1

printf '4. Commit a post on primary and obtain its post-commit WAL token.\n'
written=$(query primary app -v body='db-test post written while replica1 was paused' -f "$DDIA_SQL/write.sql")
post_id=${written%%|*}
required_lsn=${written#*|}
printf '   post_id=%s required_lsn=%s\n' "$post_id" "$required_lsn"
assert_sql t primary app -v post_id="$post_id" -f "$DDIA_SQL/visible.sql"
assert_sql f replica1 app -v post_id="$post_id" -f "$DDIA_SQL/visible.sql"
assert_sql f replica1 app -v required_lsn="$required_lsn" -f "$DDIA_SQL/caught-up.sql"

printf '5. Wait for replica2 and verify the same post is visible.\n'
wait_for_sql t replica2 app -v required_lsn="$required_lsn" -f "$DDIA_SQL/caught-up.sql"
assert_sql t replica2 app -v post_id="$post_id" -f "$DDIA_SQL/visible.sql"

printf '6. Resume replica1 and wait for both its position and the visible post.\n'
resume_replica replica1
pause_owned=0
wait_for_sql t replica1 app -v required_lsn="$required_lsn" -f "$DDIA_SQL/caught-up.sql"
assert_sql t replica1 app -v post_id="$post_id" -f "$DDIA_SQL/visible.sql"
for node in replica1 replica2; do
    assert_sql 'not paused' "$node" admin -f "$DDIA_SQL/pause-state.sql"
done
printf 'PASS: lag reproduced and recovered; both replicas are running. Test posts remain for inspection.\n'
