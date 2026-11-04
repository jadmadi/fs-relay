# Evidence: Complete Sila Onboarding

Date: 2026-11-03
Agent: opencode (session sess-db2a7f53c5ac1dc6a1d4a34e33963505)

## What was done

1. Provisioned sila wiring with `sila heal`. It reported 1 automatic fix
   (rules, 2 MCP configs, OpenCode plugin, completion goal) and applied it.
2. Inventoried legacy surfaces with `sila onboard --json`: 3 managed, 0
   unmanaged. Nothing was left unmapped.
3. Reviewed `goals/` and closed nothing stale: one goal existed, this one.
4. Recorded three architectural directives with `sila memo`, each grounded in
   repository files:
   - errno travels as a number in `error.code`; HTTP statuses stay 200/400/500
   - only `internal/local` touches the filesystem; `internal/wire` imports no
     `net/http`, go-fuse or `os`
   - five packages, standard library first, process-local inode map
5. Recorded one lesson with `sila lesson`: a fork push returns 403 when
   `GITHUB_TOKEN` lacks write access, and `env -u GITHUB_TOKEN` fixes it by
   falling back to the keyring credential.
6. Verified with `sila onboard check --strict`.

## Verification

| Check | Result |
|-------|--------|
| `sila heal` | applied 1 automatic fix |
| `sila onboard --json` | `unmanaged: 0`, `managed: 3` |
| `sila memo` count | 3 recorded |
| `sila lesson` count | 1 recorded |
| `sila onboard check --strict` | exit 0, 13 pass, 0 warn |
| `just check` | exit 0 on the synced `main` |

## Commands

```
sila heal
sila onboard --json
sila onboard check --strict   # exit 0, 13 pass, 0 warn
just check                    # exit 0
```

Verdict: PASS
