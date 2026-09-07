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
