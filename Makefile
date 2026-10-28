SHELL := /bin/bash
export PATH := $(CURDIR)/.tools/go/bin:$(PATH)
SQLC := $(CURDIR)/.tools/bin/sqlc
.PHONY: bootstrap generate test check build frontend fmt
bootstrap:
	./tools/bootstrap.sh
generate:
	cd backend && $(SQLC) generate
	cd agent && $(SQLC) generate
test: generate
	cd shared && go test ./... -count=1
	cd backend && go test ./... -count=1
	cd agent && go test ./... -count=1
	npm test --prefix frontend
check: test
	cd shared && go vet ./...
	cd backend && go vet ./...
	cd agent && go vet ./...
	npm run typecheck --prefix frontend
	npm run build --prefix frontend
build: generate frontend
	mkdir -p bin
	cd backend && CGO_ENABLED=0 go build -trimpath -o ../bin/sor-server ./cmd/server
	cd agent && CGO_ENABLED=0 go build -trimpath -o ../bin/sor-agent ./cmd/agent
frontend:
	npm run build --prefix frontend
fmt:
	find backend agent shared -name '*.go' -not -path '*/sqlc/*' -exec gofmt -w {} +
