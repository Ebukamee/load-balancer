# loadbalancer

A layer-7 (HTTP) load balancer written in Go from scratch, built in ten steps
as a learning project.

## Layout

```
cmd/lb/          load balancer entry point (flags, wiring, server start)
cmd/backend/     demo upstream server to balance across
internal/proxy/  the HTTP handler that forwards traffic
.github/         CI: gofmt, go vet, tests with -race, build, golangci-lint
```

`internal/` packages cannot be imported by other modules, which keeps the
public surface small while the design changes.

## Run it

```sh
go run ./cmd/backend -addr :7001 -name b1   # terminal 1
go run ./cmd/lb                             # terminal 2
curl -i localhost:8080                      # terminal 3
```

## Checks (same as CI)

```sh
gofmt -l .
go vet ./...
go test ./...
golangci-lint run
```

## Roadmap

1. Project skeleton and CI ✅
2. Reverse proxy to one backend
3. Many backends and round robin
4. Configuration file
5. Active health checks
6. Timeouts, retries and failover
7. More algorithms (least connections, weighted)
8. Observability: logs and metrics
9. Graceful shutdown and hot reload
10. Docker, load testing and release
