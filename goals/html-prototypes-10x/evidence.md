# Evidence: Elevate html-prototypes 10x and align welcome page

Date: 2026-11-05
Goal: `html-prototypes-10x`
Scope: `.ui-craft`, `html-prototypes`, `cmd/fs-relay/root.go`,
`cmd/fs-relay/root_test.go`.

## What was built

1. Tokens and brief: updated `.ui-craft/brief.md` and `.ui-craft/tokens.md` to
   allow progressive-enhancement interactivity, locked font weights to 400,
   600, and 700, and formalized radius, spacing, and surface tokens.
2. Responsive shell and style hygiene: refactored `html-prototypes/style.css`
   to remove hardcoded pixel radii and ad-hoc sizes, clamped the `76rem`
   breakout margin so `.diagram` and `.chart` keep a `2rem` viewport gutter,
   and added `.scrolled-end` mobile nav scroll-mask cleanup.
3. Explainer player and Scene 5 SVG: upgraded `html-prototypes/explainer.css`,
   `html-prototypes/explainer.js`, and `html-prototypes/explainer.html` with
   GPU-accelerated `.fadeable`, `.step`, and `.flash` transitions, cubic
   ease-out progress interpolation, pointer-capture timeline scrubbing, keyboard
   shortcuts (`Space`, `K`, `J`, `L`, `ArrowLeft`, `ArrowRight`), runnable
   terminal commands with an SVG cursor in Scene 2, and a `720x400` SVG chart
   in Scene 5 so mobile viewports never clip.
4. Interactive lab (`html-prototypes/ui.js`): added progressive-enhancement
   `<pre>` copy buttons with SVG checkmark feedback, `sessionStorage` smoke
   checklists on `run.html`, fastest-first benchmark sorting plus speedup
   badges and filesystem filtering on `compare.html`, step-through diagram
   explorers on `index.html`, `wire.html`, `coherence.html`, and `locks.html`,
   and a live byte-size routing slider on `raw.html`.
5. Server welcome page: aligned `landingPage` in `cmd/fs-relay/root.go` and
   `cmd/fs-relay/root_test.go` with the ink-slate (`#0d1317`) and signal-rose
   (`#f43f5e`) token palette and quick-start mount command.
6. Interactive 3D Three.js reference lab (`interactive-prototype/`): built
   `index.html`, `style.css`, `data.js`, and `app.js` using vendored offline
   Three.js `v0.170.0` (`OrbitControls`) and the Claude Interactive Diagram
   pattern (`0 · Everyday View` showing 3D Dev Laptop, Office Desktop & Cloud
   VM sharing one host folder with zero client copies across Steps `0..2`, plus
   `1 · System Topology` and `2 · Kernel & Lock Cutaway` across Steps `3..7`
   for the 10 OS and protocol structures, horizontal non-overlapping guide and
   simulation docks, pulsing 3D beacon ring, and 8-question self-test quiz).

## Acceptance results

| Check | Result |
|-------|--------|
| AC-001 `node --check html-prototypes/explainer.js` and `ui.js` | pass (exit 0) |
| AC-002 `375x812` and `1216x800` zero horizontal overflow across all 8 pages | pass (`scrollWidth === clientWidth`) |
| AC-003 Scene 5 SVG conversion in `explainer.html` | pass (`1` `<svg>`, `0` HTML `.chart`) |
| AC-004 Progressive-enhancement lab features verified in headless Chromium | pass (copy buttons, checklists, filter, steppers, raw router) |
| AC-005 `gofmt -l .`, `go vet ./...`, `go test -race ./...` | pass (exit 0) |
| AC-006 No external URLs, CDNs, or webfonts in `html-prototypes/` | pass |
| AC-007 `node --check interactive-prototype/app.js` and `ripwire . --quality-delta` | pass (`gating=0`, 10 3D nodes verified in headless Chromium) |

## Commands

```
gofmt -l .                                                     # empty
go vet ./...                                                   # exit 0
go test ./...                                                  # exit 0
go test -race ./...                                            # exit 0
node --check html-prototypes/ui.js                             # exit 0
node --check html-prototypes/explainer.js                      # exit 0
node --check interactive-prototype/app.js                      # exit 0
ripwire . --quality-delta --legend=compact                     # gating=0
```

## Memory Writes

```json
{
  "schema": "sila/memory-writes/v1",
  "goal_slug": "html-prototypes-10x",
  "prime": {"ran": true},
  "recall": [{"query": "html-prototypes landingPage root.go ui-craft", "matches": 0}],
  "memos": [{"id": "fact_6c896f80"}],
  "lessons": [{"id": "fact_545a88bf"}],
  "cases": [{"id": "fact_528c9cfb"}],
  "facts_verify": {"exit_code": 0},
  "handoff": {"id": "hoff_7a8ef395501f"}
}
```

Verdict: PASS
