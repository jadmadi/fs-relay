set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

ldflags := "-s -w"

# production binaries in this directory
build:
    just _bin fs-relay ./cmd/fs-relay
    just _bin fs-relay-bind ./cmd/fs-relay-bind

# unit tests
test:
    go test ./...

# gofmt, vet, and unit tests
check:
    files="$(gofmt -l .)"; test -z "$files" || { printf '%s\n' "$files"; exit 1; }
    go vet ./...
    just test

# linux release binaries in dist/
release:
    mkdir -p dist
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 just _bin dist/fs-relay-linux-amd64 ./cmd/fs-relay
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 just _bin dist/fs-relay-bind-linux-amd64 ./cmd/fs-relay-bind
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 just _bin dist/fs-relay-linux-arm64 ./cmd/fs-relay
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 just _bin dist/fs-relay-bind-linux-arm64 ./cmd/fs-relay-bind

# regenerate internal/wire/fsrelay_generated.go from the schema
flatc:
    flatc --go --gen-onefile --go-namespace wire -o internal/wire internal/wire/fsrelay.fbs
    gofmt -w internal/wire/fsrelay_generated.go

[private]
_bin name pkg:
    go build -trimpath -ldflags={{ quote(ldflags) }} -o {{ name }} {{ pkg }}
