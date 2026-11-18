.PHONY: test

test:
	go test ./...

.PHONY: api-run

api-run:
	go run ./cmd/server

NODE ?= primary
ROLE ?= app
export NODE ROLE

.PHONY: db-up db-down db-status db-psql db-pause db-resume db-test db-migrate db-board-test

db-up:
	bash infra/postgres/db.sh up

db-down:
	bash infra/postgres/db.sh down

db-status:
	bash infra/postgres/db.sh status

db-psql:
	bash infra/postgres/db.sh psql

db-pause:
	bash infra/postgres/db.sh pause

db-resume:
	bash infra/postgres/db.sh resume

db-test:
	bash labs/postgres_replication/test.sh

db-migrate:
	bash infra/postgres/db.sh migrate

db-board-test:
	bash labs/postgres_replication/board-test.sh

.PHONY: load-observe load-closed load-open

# 書き込み負荷の実験。labs/postgres_write_load/README.md を参照。
THREADS ?= 1000
LEVELS ?= 1,2,4,8,16,32,64
NUMBERING ?= pessimistic
LOAD_ARGS ?=

load-observe:
	bash labs/postgres_write_load/run.sh -mode observe -duration 10s $(LOAD_ARGS)

load-closed:
	bash labs/postgres_write_load/run.sh -mode closed -threads $(THREADS) -levels $(LEVELS) -numbering $(NUMBERING) $(LOAD_ARGS)

load-open:
	bash labs/postgres_write_load/run.sh -mode open -threads $(THREADS) -rates $(RATES) -pool $(POOL) -numbering $(NUMBERING) $(LOAD_ARGS)
