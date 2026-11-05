# Token Spine — fs-relay html-prototypes

Three layers per ui-craft tokens.md. Single dark theme,
by brief §5 — these values are the design, not a remap.
Ramps are trimmed to steps the surface uses. Colors are
hex, not OKLCH: hand-authored SVG attributes need exact
values, and the scales below are perceptually uniform
by construction (ink ramp) or proven in use (rose).

## Layer 1: primitives

```css
/* Neutrals — ink slate, near-zero chroma */
--gray-50:  #eef2f4;
--gray-200: #c3ced4;
--gray-400: #8b98a1;
--gray-600: #4d5a62;
--gray-800: #232e34;
--gray-900: #161e23;
--gray-950: #0d1317;

/* Accent — signal rose */
--accent-200: #fecdd3;
--accent-300: #fda4af;
--accent-400: #fb7185;
--accent-500: #f43f5e;
--accent-700: #be123c;
--accent-900: #4c0519;

/* Spacing — 8pt with quarter/half steps */
--space-2xs: 0.125rem;
--space-xs: 0.25rem;
--space-sm: 0.5rem;
--space-md: 1rem;
--space-lg: 1.5rem;
--space-xl: 2rem;
--space-2xl: 3rem;
--space-3xl: 4rem;
--space-4xl: 6rem;

/* Type — strict scale & 3 weights (Pass 2) */
--text-xs: 0.75rem;
--text-sm: 0.875rem;
--text-base: 1rem;
--text-lg: 1.125rem;
--text-xl: 1.25rem;
--text-2xl: 1.5rem;
--text-3xl: 1.875rem;
--text-4xl: 2.25rem;
--text-5xl: 3rem;
--font-regular: 400;
--font-semibold: 600;
--font-bold: 700;
--leading-tight: 1.05;
--leading-snug: 1.3;
--leading-normal: 1.5;
--leading-relaxed: 1.65;
--tracking-display: -0.035em;
--tracking-normal: 0;
--tracking-wide: 0.08em;

/* Radii — varied by element role (Pass 9) */
--radius-xs: 0.1875rem;
--radius-sm: 0.25rem;
--radius-md: 0.375rem;
--radius-lg: 0.625rem;
--radius-xl: 0.875rem;
--radius-pill: 9999px;

/* Depth — dark uses rings + ambient shadow */
--ring-subtle: inset 0 1px 0 rgb(255 255 255 / 0.06);
--shadow-raised: 0 1px 2px rgb(0 0 0 / 0.35), 0 8px 24px rgb(0 0 0 / 0.28);

/* Motion — custom ease-out, GPU-only (Pass 7) */
--duration-fast: 140ms;
--duration-base: 220ms;
--ease-out: cubic-bezier(0.22, 1, 0.36, 1);

/* Z-index */
--z-sticky: 10;
--z-skip: 20;

/* Layout */
--measure: 65ch;
--shell-sm: 46rem;
--shell-lg: 60rem;
```

## Layer 2: semantics

```css
--surface-canvas: var(--gray-950);
--surface-raised: var(--gray-900);
--surface-overlay: var(--gray-800);
--surface-sunken: #080d10;
--surface-glow: rgb(244 63 94 / 0.10);
--text-primary: var(--gray-50);
--text-secondary: var(--gray-200);
--text-tertiary: var(--gray-400);
--text-faint: var(--gray-600);
--border-default: var(--gray-800);
--border-subtle: rgb(255 255 255 / 0.08);
--border-focus: var(--accent-500);
--accent: var(--accent-500);
--accent-soft: var(--accent-300);
--accent-deep: var(--accent-700);
--accent-wash: #2b1119;
--chart-them: var(--gray-600);
--chart-floor: var(--gray-200);
--font-display: system-ui, "Segoe UI", "Avenir Next", sans-serif;
--font-body: Charter, "Bitstream Charter", "Sitka Text", Cambria, Georgia, serif;
--font-mono: ui-monospace, "SF Mono", "Cascadia Mono", Menlo, Consolas, monospace;
```

## Layer 3: components

```css
/* Verdict panel — the signature component */
--verdict-bg: var(--accent-wash);
--verdict-border: var(--accent-deep);
--verdict-q: var(--accent-soft);
```

Any color, radius, or spacing value not drawn from these layers is off-system.
