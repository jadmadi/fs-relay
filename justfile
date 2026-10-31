set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

ldflags := "-s -w"

# production binaries in this directory
build:
    just _bin fs-relay ./cmd/fs-relay
    just _bin fs-relay-bind ./cmd/fs-relay-bind

# regenerate internal/wire/fsrelay_generated.go from the schema
flatc:
    flatc --go --gen-onefile --go-namespace wire -o internal/wire internal/wire/fsrelay.fbs
    gofmt -w internal/wire/fsrelay_generated.go

[private]
_bin name pkg:
    go build -trimpath -ldflags={{ quote(ldflags) }} -o {{ name }} {{ pkg }}
