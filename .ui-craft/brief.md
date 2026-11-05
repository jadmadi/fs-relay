# Design Brief — fs-relay html-prototypes

## 1. Product purpose

Explains fs-relay — what it is, how to run it, and how it works inside — through eight interactive-capable illustrated pages, an animated explainer, and the server's built-in welcome page (`GET /`).

## 2. Primary user

A sysadmin evaluating or operating fs-relay on a laptop, deciding whether to mount it; plus fs-relay developers, who read the internals pages and benchmarks.

## 3. Principles

Ranked: when two apply, the higher wins.

1. "Plays anywhere, needs nothing." Self-contained files over anything a network, account, or build step provides. Kills CDNs, frameworks, and webfonts on sight. Every page renders in full without JavaScript and layers interactive exploration when JavaScript is enabled.
2. "The answer leads; evidence follows." Every page opens with its verdict, then key lines, then proof. Kills narrative buildup and buried conclusions.
3. "Numbers carry citations." Every published figure traces to README.md, Benchmark.md, or ROADMAP.md. Kills unsourced claims.
4. "The instrument journal, not the brochure." Grotesk display, serif prose, one signal accent; data first, interactive inspection where it clarifies mechanics, no marketing patterns. Kills heroes, card grids, and decoration without a job.

## 4. Success metric

A first-time reader can state what fs-relay is, copy and run the commands, inspect the live behavior on the diagrams, and verify how fast it is within five minutes on the Overview, Run, and Compare pages.

## 5. Out of scope

- Does not teach FUSE, HTTP, or Linux from zero.
- Does not replace SPEC.md, README.md, or ROADMAP.md as contracts.
- Does not collect analytics, phone home, or require an account.
- Does not ship a light theme; the identity is single-theme dark.

## 6. Learned constraints

- **2026-11-05** — New visual identity, not a refinement of ember-on-coal. *Why:* the user chose "bold new identity" over the recommended "refined dark"; distinctness outranks brand continuity for this surface.
- **2026-11-05** — Option 3 (Full unify + interactive lab). *Why:* progressive-enhancement interactivity (`html-prototypes/ui.js` for copy buttons, checkable smoke lists, sortable charts with speedup badges, and step-through diagram explorers) plus syncing `cmd/fs-relay/root.go` with the ink-slate and signal-rose token system makes both the docs and the binary feel like one instrument.
