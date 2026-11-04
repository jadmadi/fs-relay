set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

version := `git describe --tags --always --dirty 2>/dev/null || echo devel`
ldflags := "-s -w -X main.version=" + version

# production binaries in this directory
build:
    just _bin fs-relay ./cmd/fs-relay
    just _bin fs-relay-bind ./cmd/fs-relay-bind

# unit tests
test:
    go test ./...

# unit tests under the race detector
race:
    go test -race ./...

# gofmt, vet, and unit tests
check:
    files="$(gofmt -l .)"; test -z "$files" || { printf '%s\n' "$files"; exit 1; }
    go vet ./...
    just test

# name and package, one pair per line. Both release tarballs contain every one of these.
bins := "
fs-relay ./cmd/fs-relay
fs-relay-bind ./cmd/fs-relay-bind
"

# linux tarballs in dist/, one per architecture
release:
    mkdir -p dist
    just _pack amd64
    just _pack arm64

# regenerate internal/wire/fsrelay_generated.go from the schema
flatc:
    flatc --go --gen-onefile --go-namespace wire -o internal/wire internal/wire/fsrelay.fbs
    gofmt -w internal/wire/fsrelay_generated.go

[private]
_bin name pkg:
    go build -trimpath -ldflags={{ quote(ldflags) }} -o {{ name }} {{ pkg }}

[private]
_pack arch:
    #!/usr/bin/env bash
    set -euo pipefail
    stage="dist/linux-{{arch}}"
    rm -rf "$stage"
    mkdir -p "$stage"
    names=()
    while read -r name pkg; do
        [ -n "$name" ] || continue
        CGO_ENABLED=0 GOOS=linux GOARCH={{arch}} just _bin "$stage/$name" "$pkg"
        names+=("$name")
    done <<< {{ quote(bins) }}
    tar -C "$stage" -czf "dist/fs-relay-linux-{{arch}}.tar.gz" "${names[@]}"
    rm -rf "$stage"
