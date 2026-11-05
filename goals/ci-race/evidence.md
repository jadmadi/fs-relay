# Evidence: Run the race detector in CI

Date: 2026-11-04
Goal: `ci-race`
Issue: #21

## What changed

| File | Change |
|------|--------|
| `justfile` | New `race` recipe: `go test -race ./...` |
| `.github/workflows/ci.yml` | New `race` job running `just race`; `test` keeps `just check` and `just build` |
| `AGENTS.md` | "Done means" names the race run and its CI job |

The race job is separate on purpose. `-race` roughly doubles the test time, and
the fast `test` job still answers a formatting or vet mistake in under a minute.

## Verification

| Command | Result |
|---------|--------|
| `just check` | exit 0, every package ok |
| `just race` | exit 0, every package ok under `-race` |
| workflow parse | two jobs: `test`, `race` |

## Memory Writes

```json
{
  "schema": "sila/memory-writes/v1",
  "goal_slug": "ci-race",
  "prime": {"ran": true},
  "recall": [{"query": "race detector ci", "matches": 0}],
  "memos": [{"id": "fact_fab09007"}],
  "lessons": [{"id": "fact_459977b3"}],
  "cases": [],
  "facts_verify": {"exit_code": 1},
  "handoff": {"id": "session-end"}
}
```

Verdict: PASS
