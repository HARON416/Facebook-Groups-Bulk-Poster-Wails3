.DEFAULT_GOAL := help

.PHONY: help run tidy deps frontend build check test fmt

help:
	@printf '%s\n' \
	  'Postika website commands:' \
	  '  make run       Run the website (default: localhost:8080)' \
	  '  make tidy      Tidy Go module dependencies' \
	  '  make deps      Install frontend dependencies from the lockfile' \
	  '  make frontend  Compile TypeScript browser assets' \
	  '  make build     Compile browser assets and the Go executable' \
	  '  make check     Check TypeScript and run go vet' \
	  '  make test      Run Go tests' \
	  '  make fmt       Format Go source files'

run:
	go run .

tidy:
	go mod tidy

deps:
	npm ci

frontend:
	npm run build

build: frontend
	go build -o postika-website .

check:
	npm run check
	go vet ./...

test:
	go test ./...

fmt:
	go fmt ./...
