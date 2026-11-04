# Plan: Drop the namespace cache that nothing reads

Read `facts.md`. Work on `main`, then publish from a clean branch.

## Execution Steps

- [x] **T1 - Remove the dead map**
  - Goal: `Call` checks the namespace on every request, as the code already does.
  - Files: `internal/local/tree.go`.
  - Requirements: delete the `dirs` field and its initializer; delete
    `OpenNamespace`, which becomes a one-call wrapper; `Call` calls
    `CheckNamespace`; the doc comment says why the stat runs every time.
  - Acceptance: no reference to `dirs` or `OpenNamespace` remains.

- [x] **T2 - Prove the check runs per call**
  - Files: `internal/local/local_test.go`.
  - Requirements: a call succeeds, the namespace directory is removed, and the
    next call fails with `ENOENT`.
  - Acceptance: `TestCallChecksNamespaceEveryTime` passes; the old wrapper test
    becomes `TestCheckNamespace`.

- [x] **T3 - Evidence**
  - Files: `goals/namespace-cache/evidence.md`.
  - Acceptance: `gofmt -l .` empty, vet, test and race pass; evidence has
    `Verdict: PASS`.

## Definition of done

```
just check
just race
```
