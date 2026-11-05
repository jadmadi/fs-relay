# Facts: Elevate html-prototypes 10x and align server landing page

## Architectural Invariants & Constraints
- Self-contained files only: zero external CDNs, frameworks, or webfonts (`Plays anywhere, needs nothing`).
- Progressive enhancement: without JavaScript, every page and every scene in `html-prototypes/` renders its full content, diagrams, and tables cleanly.
- Token discipline: every color, spacing, type size, weight (at most 3 weights in a viewport), and radius in `html-prototypes/style.css` and `html-prototypes/explainer.css` comes from `.ui-craft/tokens.md`.
- Motion & accessibility: only `transform` and `opacity` animate; `:active` press feedback (`scale(0.97)`) on buttons; full `prefers-reduced-motion: reduce` support; keyboard navigation and visible `:focus-visible` everywhere.
- `cmd/fs-relay/root.go` keeps its exact HTTP content negotiation and health JSON contract (`TestRootNegotiates` in `cmd/fs-relay/root_test.go`) while serving the updated ink-slate and signal-rose welcome page.

## File & Interface Contracts
- Relevant files:
  - `.ui-craft/brief.md`
  - `.ui-craft/tokens.md`
  - `html-prototypes/CNAME`
  - `html-prototypes/style.css`
  - `html-prototypes/explainer.css`
  - `html-prototypes/explainer.js`
  - `html-prototypes/ui.js`
  - `html-prototypes/index.html`
  - `html-prototypes/run.html`
  - `html-prototypes/compare.html`
  - `html-prototypes/wire.html`
  - `html-prototypes/coherence.html`
  - `html-prototypes/locks.html`
  - `html-prototypes/raw.html`
  - `html-prototypes/explainer.html`
  - `cmd/fs-relay/root.go`
  - `cmd/fs-relay/root_test.go`
