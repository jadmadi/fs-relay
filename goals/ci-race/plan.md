# Plan: Run the race detector in CI

Read `facts.md`. Work on `main`, then publish from a clean branch.

## Execution Steps

- [x] **T1 - `just race` recipe**
  - Goal: one command runs the unit tests under the race detector.
  - Files: `justfile`.
  - Requirements: `race` runs `go test -race ./...`; `check` and `test` stay
    unchanged, so the fast path keeps its time.
  - Acceptance: `just race` exits 0 and prints every package.

- [x] **T2 - Separate CI job**
  - Goal: a data race fails the build.
  - Files: `.github/workflows/ci.yml`.
  - Requirements: a `race` job next to `test`, same checkout and Go setup,
    running `just race`. The `test` job keeps `just check` and `just build`.
  - Acceptance: the workflow parses and lists two jobs.

- [x] **T3 - Document the gate**
  - Files: `AGENTS.md`, `goals/ci-race/evidence.md`.
  - Requirements: "Done means" names the race run and where it runs.
  - Acceptance: evidence has `Verdict: PASS`; `just check` and `just race`
    pass locally.

## Definition of done

```
just check
just race
```
