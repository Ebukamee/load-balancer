# Shortcuts for the commands CI runs. On Windows without make, copy the
# command after the target name into PowerShell.
.PHONY: run backend test race lint fmt build

run:
	go run ./cmd/lb

backend:
	go run ./cmd/backend -addr :7001 -name b1

test:
	go test ./...

race:
	go test -race ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .

build:
	go build -o bin/ ./cmd/...
