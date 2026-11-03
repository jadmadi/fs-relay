---
name: pr-triage
description: "Triage fs-relay's upstream pull requests and issues: find new maintainer responses, assignments, CI and merge state, then propose the single next action that is assigned and in scope. Use when the user asks what is pending, what to do next on fs-relay contributions, to check PRs or issues, or to triage upstream work."
---

# PR and issue triage for fs-relay

Upstream: `muayyad-alsadi/fs-relay`. Fork: `jadmadi`. GitHub login:
`jadmadi`. The user contributes from the fork; the maintainer merges.

## Hard rules

1. Work only items assigned to `@jadmadi`, plus the user's own open PRs.
2. An agreed but unassigned issue is not work. Ask for the assignment.
3. Do not merge a PR, including your own. The maintainer merges.
4. One root cause per PR. No drive-by fixes, refactors, or extras.
5. Keep at most three open PRs.
6. Do not open issues or PRs, or post comments, without the user's consent.
7. Keep PR branches free of sila files: `goals/`, `.agents/`, and the
   `AGENTS.md` block. The local pre-push hook enforces this.
8. End with one next action, never a menu.

## Commands

Run from the repository root. In this environment `gh` needs the token
unset, so prefix every call with `env -u GITHUB_TOKEN`.

### Assignments

```sh
gh issue list -R muayyad-alsadi/fs-relay --assignee jadmadi \
  --state open --json number,title
gh pr list -R muayyad-alsadi/fs-relay --assignee jadmadi \
  --state all --json number,title,state
```

### Pull requests

```sh
gh pr list -R muayyad-alsadi/fs-relay --state all --limit 10 \
  --json number,title,state,comments,updatedAt
gh pr view <n> -R muayyad-alsadi/fs-relay \
  --json state,mergeStateStatus,statusCheckRollup,comments
gh pr checks <n> -R muayyad-alsadi/fs-relay
```

Read the last two comments. A `mergeStateStatus` of `DIRTY` means the branch
is behind or conflicts. `UNSTABLE` means a non-required check is pending.

### Issues

```sh
gh issue list -R muayyad-alsadi/fs-relay --state all --limit 30 \
  --json number,title,state,assignees,comments,updatedAt
gh issue view <n> -R muayyad-alsadi/fs-relay --json comments
```

### Invitations

```sh
gh api /user/repository_invitations
```

## Classification

| Finding | Meaning | Action |
|---------|---------|--------|
| Open issue assigned to you | Your work | Implement it |
| Agreed, not assigned | Maintainer said yes | Ask for assignment |
| No maintainer response | Unknown | Wait |
| Your PR `DIRTY` | Behind or conflicts | Rebase, test, force-push |
| Your PR `UNSTABLE` | Non-required check pending | Ignore if `test` passes |
| Bot review | CodeRabbit, PR-Agent | Ignore unless the maintainer confirms |
| Unanswered question | Maintainer is blocked | Answer first |

Bot reviews in this repo are noise. The maintainer said the PR-Agent
findings were irrelevant and CodeRabbit skips non-default bases.

## Priority

1. Answer an open maintainer question. It unblocks them.
2. Rebase a `DIRTY` PR of yours and keep its required check green.
3. Ask for the assignment of agreed items.
4. Implement assigned items.
5. Optional follow-ups only when assigned.

## Scope test

Before proposing any work, answer these:

- Which assigned issue or own PR does it map to?
- Does it touch only the files that item names?
- Does it finish the request, not a slice of it?
- Does it add an unrequested feature, refactor, or follow-up?

If any answer fails, the action is "ask", not "implement". Never descope a
request to make it easy, and never widen it to nearby issues.

## Output

Use a Decision Brief:

1. One-line verdict: what is blocked and on whom.
2. A compact table: item, state, next action.
3. One line: the next action.

Keep it under one screen. No background prose unless asked.

## Implementation checks

When an assigned item moves to code:

```sh
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
git status --short
```

A behaviour change also updates `SPEC.md` and `ROADMAP.md`. A PR branch is
built from `upstream/main` and carries code, tests, and docs only.
