# Plan: Elevate html-prototypes 10x and align server landing page

Read `facts.md`.

## Execution Steps

- [x] **T1 - Update design brief and token spine**
  - Goal: permit progressive-enhancement interactive controls in `.ui-craft/brief.md` and lock strict token scales (3 weights, radii, type sizes, surface tokens) in `.ui-craft/tokens.md`.
  - Files: `.ui-craft/brief.md`, `.ui-craft/tokens.md`.
  - Acceptance: every CSS variable in `style.css` and `explainer.css` traces to `.ui-craft/tokens.md`.

- [x] **T2 - Harden `style.css` layout, tokens, mobile nav, and interactive component styles**
  - Goal: eliminate off-scale magic numbers, fix the `76rem` negative-margin viewport collision with `min()` breakout math, add mobile nav overflow fade masks, style copy buttons, interactive checklists, speedup badges, and diagram steppers.
  - Files: `html-prototypes/style.css`.
  - Acceptance: zero horizontal overflow at `320px`, `375px`, `768px`, `1216px` (`76rem`), and `1280px`; all sizes and radii use token variables.

- [x] **T3 - Fix `explainer.css`, `explainer.js`, and `explainer.html` (Scene 5 SVG, transitions, scrubbing, caret)**
  - Goal: add GPU `opacity`/`transform` transitions to `.fadeable`, `.step`, and `.flash`, `:active` `scale(0.97)` press feedback, cubic-bezier ease-out interpolation in `explainer.js`, pointer-capture drag scrubbing on `#progress`, Space/Arrow/K keyboard shortcuts, real `.tty-caret` element in Scene 2, runnable command in Scene 2, and a non-clipping SVG chart in Scene 5.
  - Files: `html-prototypes/explainer.css`, `html-prototypes/explainer.js`, `html-prototypes/explainer.html`.
  - Acceptance: Scene 5 fits 16:9 at `375px` with zero clipping; dragging `#progress` scrubs smoothly; `.fadeable` transitions smoothly.

- [x] **T4 - Build progressive-enhancement `html-prototypes/ui.js` and upgrade all 7 documentation pages**
  - Goal: add `ui.js` and enhance `index.html`, `run.html`, `compare.html`, `wire.html`, `coherence.html`, `locks.html`, and `raw.html` with:
    1. Auto-scrolling active mobile nav item into view.
    2. One-click copy buttons with SVG checkmark confirmation on `<pre>` command blocks in `run.html` and `raw.html`.
    3. Interactive checkable smoke checklist items with `sessionStorage` state and progress counter in `run.html`.
    4. Fastest-first bar sorting, inline speedup multiplier badges (`13× vs s3fs`, `45× vs S3`), and interactive sort/highlight controls in `compare.html`.
    5. Interactive step-through diagram explorers in `index.html`, `wire.html`, `coherence.html`, `locks.html`, and `raw.html` (including a live byte-size router slider on `raw.html`).
  - Files: `html-prototypes/ui.js`, `html-prototypes/index.html`, `html-prototypes/run.html`, `html-prototypes/compare.html`, `html-prototypes/wire.html`, `html-prototypes/coherence.html`, `html-prototypes/locks.html`, `html-prototypes/raw.html`.
  - Acceptance: pages work without JS and gain full interactive controls with JS enabled; zero console errors.

- [x] **T5 - Align `cmd/fs-relay/root.go` welcome page and unit tests**
  - Goal: update `landingPage` in `cmd/fs-relay/root.go` from the legacy ember-on-coal palette to the ink-slate (`#0d1317`) and signal-rose (`#f43f5e`) instrument-journal identity with quick-start mount instructions, and verify in `cmd/fs-relay/root_test.go`.
  - Files: `cmd/fs-relay/root.go`, `cmd/fs-relay/root_test.go`.
  - Acceptance: `gofmt -l .` is empty, `go vet ./...`, `go test ./...`, and `go test -race ./...` pass.

- [x] **T6 - Visual & DOM verification across viewports and certification**
  - Goal: verify rendered pages and Go tests, write `goals/html-prototypes-10x/evidence.md` with `Verdict: PASS`, record Sila lessons/cases, commit with `(goals/html-prototypes-10x/goal.md)`, certify, and reconcile `goals/README.md`.
  - Files: `goals/html-prototypes-10x/evidence.md`, `goals/README.md`.
  - Acceptance: `sila goals --json` reports `status: "implemented"`, `verification_status: "verified"`, `unverified_count: 0`, and `git status` is clean.
