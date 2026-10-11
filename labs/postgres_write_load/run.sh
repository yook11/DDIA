#!/usr/bin/env bash
# 書き込み負荷の1回分を実行する: 結果ディレクトリの作成、コンテナのCPU記録、loadgen。
# 引数はそのまま loadgen に渡す。
# LOADGEN_AT=network（既定）は Compose ネットワーク内のコンテナから、host はホストから接続する。
set -Eeuo pipefail
LAB=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "$LAB/../../infra/postgres/common.sh"

# Public, local-lab-only credentials from infra/postgres/compose.yaml.
: "${PRIMARY_DATABASE_URL:=postgres://ddia_app:ddia_local_app@127.0.0.1:15432/ddia?sslmode=disable}"
: "${ADMIN_DATABASE_URL:=postgres://ddia_admin:ddia_local_admin@127.0.0.1:15432/ddia?sslmode=disable}"
export PRIMARY_DATABASE_URL ADMIN_DATABASE_URL

containers=$(compose ps -q)
[[ -n "$containers" ]] || fail 'The database is not running. Start it with: make db-up'

LOADGEN_AT=${LOADGEN_AT:-network}
case "$LOADGEN_AT" in network|host) ;; *) fail 'LOADGEN_AT must be network or host.' ;; esac

run_id=$(date +%Y%m%d-%H%M%S)
out="$LAB/results/$run_id"
mkdir -p "$out"
# shellcheck disable=SC2086
docker inspect --format '{"name":"{{.Name}}","nano_cpus":{{.HostConfig.NanoCpus}},"memory":{{.HostConfig.Memory}}}' \
    $containers > "$out/containers.jsonl"

# docker stats は1回の取得に約2秒かかる。CPU律速かどうかの確認用で、細かい時系列には使わない。
(
    while true; do
        # shellcheck disable=SC2086
        docker stats --no-stream \
            --format "{\"t\":$(date +%s),\"name\":\"{{.Name}}\",\"cpu\":\"{{.CPUPerc}}\",\"mem\":\"{{.MemUsage}}\",\"block_io\":\"{{.BlockIO}}\"}" \
            $containers || true
    done
) > "$out/docker_stats.jsonl" 2>/dev/null &
stats_pid=$!

cleanup() {
    { kill "$stats_pid" && wait "$stats_pid"; } 2>/dev/null || true
}
trap cleanup EXIT

printf 'results: %s (loadgen at %s)\n' "$out" "$LOADGEN_AT"
cd "$DDIA_ROOT"
if [[ "$LOADGEN_AT" == host ]]; then
    go run ./cmd/loadgen -out "$out" "$@"
else
    GOOS=linux GOARCH=$(go env GOARCH) CGO_ENABLED=0 go build -o "$LAB/bin/loadgen" ./cmd/loadgen
    compose run --rm --no-deps loadgen \
        -out "/results/$run_id" "$@"
fi
