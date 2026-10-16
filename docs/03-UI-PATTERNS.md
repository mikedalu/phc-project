# UI Patterns

> **Project:** Design and Implementation of an Offline-Capable Web-Based Patient Records and Referral Management System for Primary Healthcare Centres
> **Document status:** Canonical. If code disagrees with this document, one of them is a bug.
> **Last updated:** Sprint 0
> **Depends on:** `00-ARCHITECTURE.md` §4.3 (state management rule), §4.4 (write path), §7.2 (money)
> **Companion files you will create:** `web/src/styles/tokens.css`, `web/src/styles/print.css`

This document is written to be typed by hand. Every colour, size, duration and class name below is a real value you can paste. Where a decision looks arbitrary, the reason is stated — you will be asked "why this and not something else?" in your defence.

---

## 1. Design principles

Six principles. When two of them conflict, the one with the lower number wins.

### 1.1 Speed of data entry beats visual flourish

A registration clerk at a busy PHC enters **200 patients a day**. At that volume, 400 ms of animation per save costs over a minute of pure waiting per shift, and — far worse — it breaks the operator's rhythm. Every interaction in this app is therefore designed so the *fastest possible path* is also the default path: autofocus on the first empty field, Enter to submit, Tab order that matches the paper form, no confirmation dialog for reversible actions. Motion is used only to explain a change that already happened (a row entering a list, a panel opening), never to stage it. If you ever have to choose between an animation and 100 ms of latency, delete the animation.

### 1.2 Every state is visible — including sync state

An offline-first app has states that a server-backed app never has to explain: *this record exists on this laptop and nowhere else yet*. If the user cannot see that, they will either distrust the system (and keep a paper register as well, defeating the purpose) or over-trust it (and assume a record is safe on a server that has never seen it). So the rule is: **at every moment, at every level, the user must be able to answer "where is this record?"** — the top bar answers it for the facility (`SyncStatusIndicator`), each row answers it for itself (`SyncBadge`), and each form answers it for the record just saved ("Saved on this device · waiting to sync"). See §4.

### 1.3 Offline is normal, not an error state

The entire premise of this project (`00-ARCHITECTURE.md` §1) is that a PHC must keep working when the link is down. A UI that turns red and shouts "NO CONNECTION" every time the modem drops is a UI that lies about the system's design: it treats the normal operating mode as a failure. **Colour carries meaning here.** Red is reserved for things that are actually wrong — a failed validation, a rejected sync operation, a conflict the user must resolve. Being offline is none of those. It is neutral, occasionally warning-coloured when a limit approaches, and it *never* blocks a clinical action. A nurse recording a temperature at 03:00 with no signal must see a calm, quiet indicator — not an alarm.

### 1.4 Light mode only, permanently

There is **no dark mode and no theme toggle**. This is a deliberate product decision, not an unfinished feature, and you should state the three reasons in your defence:

1. **Uniform printing.** Referral slips and receipts are printed from these screens. A dark UI invites dark-styled preview components, and a half-considered print stylesheet that inherits dark backgrounds wastes thermal paper and produces unreadable slips. One theme means one print path.
2. **Glare in clinical rooms.** PHC consulting rooms are lit brightly, often with a window behind the operator. On a glossy laptop panel, a dark theme at 40 % brightness becomes a mirror. Light backgrounds with dark text hold up under glare far better.
3. **Halved testing surface.** Every component would need two visual states, two contrast audits, two sets of screenshots, and a print variant of each. For a one-student project, that cost buys nothing clinically.

The corollary is that the app chrome is light too: the sidebar is white on a `grey-50` app background, not a "dark sidebar". A dark sidebar is dark mode wearing a hat, and it fails reason 2.

### 1.5 One primary action per screen

Clinical staff work under time pressure and are frequently interrupted mid-task. A screen with three equally-weighted buttons makes the user read all three. Each screen therefore has **exactly one** primary (filled) button — Save, Collect payment, Dispense, Issue referral — plus secondary (outlined) and ghost (text) actions for everything else. Destructive actions are never primary and never adjacent to primary. This is why `Button` has exactly four variants and no more.

### 1.6 Numbers are data, not decoration

This app is full of numbers that get compared down a column: blood pressures, temperatures, weights, drug quantities, money in minor units (`00-ARCHITECTURE.md` §7.2). Proportional figures make `111` narrower than `888`, so a vitals column becomes a ragged sawtooth that hides outliers. Therefore: **every number in a table, every vital sign, and every money value uses tabular figures** (`font-variant-numeric: tabular-nums`), is right-aligned or decimal-aligned, and is rendered by a formatter — never by string concatenation. Money is never rendered from a float; it is `amount_minor / 100` formatted through one shared helper. §2.4 gives the tokens.

---

## 2. Design tokens

### 2.1 Why tokens live in CSS, not a JS config

Tailwind CSS v4 is CSS-first: the `@theme` block *is* the config, and every token you declare there becomes a utility (`--color-primary` → `bg-primary`, `text-primary`, `border-primary`). There is **no `tailwind.config.js`** in this project. Two rules follow:

- No inline hex colours in components. If you need a colour that is not a token, add a token.
- No arbitrary values like `bg-[#0F766E]`. That is a hex colour wearing a disguise.

### 2.2 `web/src/styles/tokens.css`

Type this file. It is the single source of truth for §2.

```css
/* web/src/styles/tokens.css
   Imported once, after tailwindcss, in web/src/app/main.css:
     @import "tailwindcss";
     @import "./styles/tokens.css";
     @import "./styles/print.css";
*/
@theme {
  /* ---------- Neutral ramp (cool grey, pinned so a Tailwind bump cannot move it) ---------- */
  --color-grey-50:  #F8FAFC;   /* app background, zebra rows, skeletons */
  --color-grey-100: #F1F5F9;   /* sunken surfaces, disabled fills, table header */
  --color-grey-200: #E2E8F0;   /* DECORATIVE borders only: card outline, row rule */
  --color-grey-300: #CBD5E1;   /* dividers on grey-50, chart gridlines */
  --color-grey-400: #94A3B8;   /* muted icons, placeholder-ish text, disabled text */
  --color-grey-450: #7C8BA1;   /* CONTROL BOUNDARY. 3.46:1 on white. See 2.3 */
  --color-grey-500: #64748B;   /* secondary body text (4.76:1) */
  --color-grey-600: #475569;   /* body text on grey-100 */
  --color-grey-700: #334155;   /* strong body text */
  --color-grey-800: #1E293B;   /* headings */
  --color-grey-900: #0F172A;   /* primary text, sidebar-active, toast surface */
  --color-grey-950: #020617;   /* print fallback, never a surface */

  /* ---------- ONE brand colour: calm clinical teal ---------- */
  --color-primary-50:  #F0FDFA;
  --color-primary-100: #CCFBF1;  /* primary-soft */
  --color-primary-200: #99F6E4;  /* soft border, selected row outline */
  --color-primary-300: #5EEAD4;  /* chart series, focus ring on dark surfaces */
  --color-primary-600: #0D9488;  /* DECORATIVE ONLY - 3.74:1 with white, fails AA text */
  --color-primary-700: #0F766E;  /* THE primary. 5.47:1 with white */
  --color-primary-800: #115E59;  /* primary hover. 7.51:1 with white */
  --color-primary-900: #134E4A;  /* primary active/pressed. 9.48:1 with white */

  --color-primary:        var(--color-primary-700);
  --color-primary-hover:  var(--color-primary-800);
  --color-primary-active: var(--color-primary-900);
  --color-primary-soft:   var(--color-primary-100);
  --color-primary-fg:     var(--color-primary-900); /* text on primary-soft: 8.41:1 */

  /* ---------- Semantic ---------- */
  --color-success:      #15803D;  /* 5.02:1 with white */
  --color-success-soft: #DCFCE7;
  --color-success-fg:   #14532D;  /* on success-soft: 8.30:1 */
  --color-warning:      #B45309;  /* 5.02:1 with white */
  --color-warning-soft: #FEF3C7;
  --color-warning-fg:   #78350F;  /* on warning-soft: 8.15:1 */
  --color-danger:       #B91C1C;  /* 6.47:1 with white */
  --color-danger-soft:  #FEE2E2;
  --color-danger-fg:    #7F1D1D;  /* on danger-soft: 8.20:1 */
  --color-info:         #1D4ED8;  /* 6.70:1 with white */
  --color-info-soft:    #DBEAFE;
  --color-info-fg:      #1E3A8A;  /* on info-soft: 8.49:1 */

  /* ---------- Surfaces, text, borders ---------- */
  --color-canvas:        var(--color-grey-50);
  --color-surface:       #FFFFFF;
  --color-surface-sunken: var(--color-grey-100);
  --color-border:        var(--color-grey-200); /* decorative: 1.23:1, allowed (not a control) */
  --color-border-strong: var(--color-grey-450); /* controls: 3.46:1, meets WCAG 1.4.11 */
  --color-text:          var(--color-grey-900); /* 17.85:1 on white */
  --color-text-muted:    var(--color-grey-500); /* 4.76:1 on white  */
  --color-text-subtle:   var(--color-grey-600); /* 7.58:1 on white  */
  --color-text-inverse:  #FFFFFF;
  --color-ring:          var(--color-primary-700); /* 5.47:1 on white, 5.25:1 on grey-50 */

  /* ---------- Typography ---------- */
  --font-sans: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue",
               "Noto Sans", Arial, sans-serif;
  --font-mono: ui-monospace, "Cascadia Mono", "SF Mono", Menlo, Consolas,
               "Liberation Mono", monospace;
  /* Numeric stack: fonts with true tabular figures. Segoe UI, SF and Roboto all
     carry a `tnum` feature; the fallback list keeps metrics stable across OSes. */
  --font-numeric: "Segoe UI", system-ui, Roboto, "Helvetica Neue", Arial, sans-serif;

  --text-xs:   0.75rem;  --text-xs--line-height:   1rem;    /* 12px / 16px  dense table meta */
  --text-sm:   0.8125rem;--text-sm--line-height:   1.125rem;/* 13px / 18px  table body, badges */
  --text-base: 0.875rem; --text-base--line-height: 1.3125rem;/*14px / 21px  THE body size */
  --text-md:   1rem;     --text-md--line-height:   1.5rem;  /* 16px / 24px  form inputs (>=16px avoids iOS zoom) */
  --text-lg:   1.125rem; --text-lg--line-height:   1.625rem;/* 18px / 26px  card titles */
  --text-xl:   1.25rem;  --text-xl--line-height:   1.75rem; /* 20px / 28px  section headings */
  --text-2xl:  1.5rem;   --text-2xl--line-height:  2rem;    /* 24px / 32px  page titles */
  --text-3xl:  1.875rem; --text-3xl--line-height:  2.25rem; /* 30px / 36px  queue counts */
  --text-4xl:  2.25rem;  --text-4xl--line-height:  2.5rem;  /* 36px / 40px  kiosk / ward board */

  --font-weight-normal:   400;
  --font-weight-medium:   500;
  --font-weight-semibold: 600;
  --font-weight-bold:     700;

  --leading-tight:  1.2;   /* display numbers */
  --leading-snug:   1.35;  /* headings */
  --leading-normal: 1.5;   /* body */
  --leading-relaxed:1.6;   /* long clinical notes */

  --tracking-tight:  -0.01em; /* >=24px headings only */
  --tracking-normal: 0;
  --tracking-wide:   0.02em;  /* uppercase labels */

  /* ---------- Spacing: one 4px base, plus named layout gutters ---------- */
  --spacing: 0.25rem;        /* p-4 = 16px, p-6 = 24px, gap-2 = 8px ... */
  --spacing-gutter:   1.5rem;   /* 24px page gutter */
  --spacing-field:    0.625rem; /* 10px vertical rhythm between form fields */
  --spacing-control:  2.25rem;  /* 36px default control height */
  --spacing-touch:    2.75rem;  /* 44px minimum interactive target */

  /* ---------- Radii: restrained. Clinical software is not a phone app. ---------- */
  --radius-xs:  0.125rem; /* 2px  tags, pills-in-table */
  --radius-sm:  0.25rem;  /* 4px  inputs, buttons, badges */
  --radius-md:  0.375rem; /* 6px  cards, popovers */
  --radius-lg:  0.5rem;   /* 8px  modals, drawers */
  --radius-full: 9999px;  /* avatars, status dots only */

  /* ---------- Shadows: three levels, all subtle ---------- */
  --shadow-sm: 0 1px 2px 0 rgb(15 23 42 / 0.05);
  --shadow-md: 0 1px 3px 0 rgb(15 23 42 / 0.08), 0 1px 2px -1px rgb(15 23 42 / 0.06);
  --shadow-lg: 0 8px 24px -6px rgb(15 23 42 / 0.12), 0 2px 6px -2px rgb(15 23 42 / 0.08);

  /* ---------- Border widths ---------- */
  --border-hairline: 1px; /* every default border */
  --border-thick:    2px; /* focus rings, active tab underline, selected row */

  /* ---------- Z-index layers (named, so no one guesses 9999) ---------- */
  --z-index-base:            0;
  --z-index-dropdown:        1000;
  --z-index-sticky:          1100;  /* sticky page header, table thead, action bars */
  --z-index-modal-backdrop:  1200;
  --z-index-modal:           1300;
  --z-index-toast:           1400;
  --z-index-tooltip:         1500;

  /* ---------- Motion: easings (Tailwind v4 has an --ease-* namespace) ---------- */
  --ease-standard: cubic-bezier(0.2, 0, 0, 1);    /* decelerate: enters, expands */
  --ease-exit:     cubic-bezier(0.4, 0, 1, 1);    /* accelerate: leaves, collapses */
  --ease-linear:   linear;                        /* progress bars, spinners ONLY */

  /* ---------- Breakpoints, pinned ---------- */
  --breakpoint-sm: 40rem;   /* 640px  */
  --breakpoint-md: 48rem;   /* 768px  */
  --breakpoint-lg: 64rem;   /* 1024px */
  --breakpoint-xl: 80rem;   /* 1280px */
  --breakpoint-2xl: 96rem;  /* 1536px */
}

/* Durations are plain custom properties: Tailwind v4 has no --duration-* theme
   namespace, and its built-in duration-100/150/200 utilities already exist.
   Declare these for CSS and for the motion (Framer Motion) variant objects. */
:root {
  --duration-instant: 100ms; /* hover, colour swaps, focus ring */
  --duration-fast:    150ms; /* dropdown, tooltip, toast, badge change */
  --duration-normal:  200ms; /* modal, drawer, row enter, panel expand - the maximum */
}

/* Numeric text utility. Apply to every table cell, vital and money value. */
@utility tnum {
  font-family: var(--font-numeric);
  font-variant-numeric: tabular-nums;
  font-feature-settings: "tnum" 1, "lnum" 1;
}
```

### 2.3 Contrast budget — every pair, measured

These are computed values (WCAG 2.1 relative-luminance formula), not estimates. **AA requires 4.5:1 for body text, 3:1 for large text and UI boundaries.**

| Foreground | Background | Ratio | Use | Verdict |
|---|---|---|---|---|
| `grey-900` | `white` | **17.85:1** | Primary text | AAA |
| `grey-800` | `white` | **14.63:1** | Headings | AAA |
| `grey-700` | `white` | **10.35:1** | Strong body text | AAA |
| `grey-600` | `white` | **7.58:1** | Table body, secondary text | AAA |
| `grey-500` | `white` | **4.76:1** | Muted labels, meta, hint text | AA |
| `grey-500` | `grey-50` | **4.55:1** | Muted text on app background | AA (just) |
| `grey-400` | `white` | 2.56:1 | ❌ **Never for text.** Muted icons and disabled only | exempt |
| `white` | `primary-700` | **5.47:1** | Primary button label | AA |
| `primary-700` | `white` | **5.47:1** | Links, active nav item | AA |
| `white` | `primary-800` (hover) | **7.51:1** | Primary hover | AAA |
| `white` | `primary-900` (active) | **9.48:1** | Primary pressed | AAA |
| `white` | `primary-600` | 3.74:1 | ❌ **Decorative only.** Never a button fill or text | fail |
| `primary-900` | `primary-100` | **8.41:1** | Soft badge text | AAA |
| `success-fg` | `success-soft` | **8.30:1** | "Synced" pill | AAA |
| `white` | `success` | **5.02:1** | Success button (rare) | AA |
| `warning-fg` | `warning-soft` | **8.15:1** | "Pending / offline" pill | AAA |
| `white` | `warning` | **5.02:1** | Warning button (rare) | AA |
| `danger-fg` | `danger-soft` | **8.20:1** | "Conflict" pill | AAA |
| `white` | `danger` | **6.47:1** | Danger button | AA |
| `info-fg` | `info-soft` | **8.49:1** | Informational callout | AAA |
| `white` | `info` | **6.70:1** | Info button (rare) | AA |
| `grey-450` | `white` | **3.46:1** | Input / select / checkbox boundary | AA (1.4.11) |
| `primary-700` | `white` | **5.47:1** | Focus ring | AAA |

**Two consequences you must not "fix" by accident:**

1. **Hover darkens, it never lightens.** White on `primary-600` is 3.74:1 and fails. If you lighten a primary button on hover "because that is what websites do", you have shipped a contrast bug. `primary-700 → 800 → 900`.
2. **`grey-200` is not a control border.** It is 1.23:1 and exists for card outlines and table rules, where the boundary is decorative and the control is identified by its label. Anything focusable gets `--color-border-strong` (3.46:1). This is why the ramp carries the unusual `grey-450` step: `grey-400` is 2.56:1 and fails 1.4.11, `grey-500` is visually heavy at scale, and nothing between them exists on the standard ramp.

Note on `grey-400` for disabled text: WCAG 1.4.3 explicitly exempts *inactive* components from contrast minimums. We still pair every disabled control with visible explanatory text (§3) so the exemption is never load-bearing.

### 2.4 Typography rules

| Context | Size token | Weight | Figures | Alignment |
|---|---|---|---|---|
| Page title | `text-2xl` | `font-semibold` | — | left |
| Section heading | `text-xl` | `font-semibold` | — | left |
| Card title | `text-lg` | `font-medium` | — | left |
| Form label | `text-base` | `font-medium` | — | left |
| Form input value | `text-md` (16px) | `font-normal` | `tnum` if numeric | left |
| Table header | `text-xs` uppercase `tracking-wide` | `font-semibold` | — | matches column |
| Table cell | `text-sm` | `font-normal` | `tnum` | left for text, **right** for numbers |
| Vitals value | `text-lg` | `font-semibold` | `tnum` | right |
| Money | `text-base` | `font-medium` | `tnum` | right, 2 decimals always |
| Hint text | `text-sm` `text-muted` | `font-normal` | — | left |
| Error text | `text-sm` `text-danger` | `font-medium` | — | left |
| Badge / pill | `text-xs` | `font-medium` | `tnum` if it contains a count | centre |
| Receipt / print | `--font-mono` | `font-normal` | fixed pitch | per §7 |

**Tabular figures are mandatory** on: every `DataTable` cell that holds a number, the vitals row, all money, all counts in `StatCard`, queue counts, ages, and durations. Enforcement is mechanical, not a matter of taste:

- `<td className="tnum text-right">` for numbers.
- Money always through one formatter: `formatMoney(minor, currency)` → `"1,500.00"`. Never `(minor/100).toFixed(2)` inline.
- Dates are rendered by `date-fns` in local time (`00-ARCHITECTURE.md` §7.3) and, where they appear in a column, use `tnum` so the digits line up even though the string is proportional.

### 2.5 Spacing, radii, shadow, border, z-index quick reference

| Token | Value | Typical use |
|---|---|---|
| `--spacing` base | 4px | `p-1`=4, `p-2`=8, `p-3`=12, `p-4`=16, `p-5`=20, `p-6`=24, `p-8`=32, `p-10`=40, `p-12`=48 |
| `gap-2` / `gap-3` | 8px / 12px | inline control spacing |
| `gap-4` / `gap-6` | 16px / 24px | field spacing / section spacing |
| `space-y-10` | 40px | between page sections |
| `--spacing-field` | 10px | between label, input and error |
| `--spacing-control` | 36px | default control height (desktop, mouse) |
| `--spacing-touch` | **44px** | minimum hit area — enforced on coarse pointers (§8) |
| `--spacing-gutter` | 24px | page horizontal padding (16px below `md`) |

| Level | Value | Applied to |
|---|---|---|
| `shadow-sm` | ambient 5 % | resting cards, table container, sticky bars |
| `shadow-md` | 1px lift | dropdowns, popovers, combobox listbox, tooltip |
| `shadow-lg` | 24px blur | modal, drawer, command palette only |

No other shadow exists. No `shadow-xl`, no coloured glows, no inner shadows.

| Layer | Value | Contents |
|---|---|---|
| `z-dropdown` | 1000 | DropdownMenu, Select content, Combobox listbox, DatePicker popover |
| `z-sticky` | 1100 | Sticky page header, `DataTable` thead, sticky form action bar, queue column headers |
| `z-modal-backdrop` | 1200 | Dialog/Drawer overlay |
| `z-modal` | 1300 | Dialog and Drawer content |
| `z-toast` | 1400 | sonner toaster region |
| `z-tooltip` | 1500 | Tooltips (must be able to escape a modal) |

### 2.6 Motion tokens

| Token | Value | Used for |
|---|---|---|
| `--duration-instant` | 100ms | hover, `active`, focus ring appearance, icon colour |
| `--duration-fast` | 150ms | dropdown/popover open, tooltip, toast, badge content swap |
| `--duration-normal` | 200ms | modal, drawer, command palette, row enter, accordion |
| `--ease-standard` | `cubic-bezier(0.2, 0, 0, 1)` | anything appearing, expanding or entering |
| `--ease-exit` | `cubic-bezier(0.4, 0, 1, 1)` | anything disappearing or collapsing |
| `--ease-linear` | `linear` | indeterminate `ProgressBar`, spinner |

**Animate `opacity` and `transform` only.** Animating `height`, `width`, `top` or `margin` forces layout on every frame; on the low-spec laptops a PHC actually owns, that is visible jank during exactly the moment the operator is typing.

```tsx
// web/src/lib/motion.ts - one place, so 200 ms stays 200 ms
import type { Transition, Variants } from 'motion/react';

export const reduced = () =>
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

export const t = {
  instant: { duration: 0.1, ease: [0.2, 0, 0, 1] },
  fast:    { duration: 0.15, ease: [0.2, 0, 0, 1] },
  normal:  { duration: 0.2, ease: [0.2, 0, 0, 1] },
  exit:    { duration: 0.15, ease: [0.4, 0, 1, 1] },
} satisfies Record<string, Transition>;

export const fadeInUp: Variants = {
  hidden: { opacity: 0, y: 4 },
  show:   { opacity: 1, y: 0, transition: t.normal },
  exit:   { opacity: 0, y: 2, transition: t.exit },
};
```

Reduced motion is handled globally, once, rather than in every component:

```tsx
// web/src/app/motion-provider.tsx
import { MotionConfig } from 'motion/react';
export const MotionProvider = ({ children }: { children: React.ReactNode }) => (
  <MotionConfig reducedMotion="user">{children}</MotionConfig>
);
```

`reducedMotion="user"` makes motion honour the OS setting automatically: transforms are dropped, opacity is kept. That is the correct behaviour — a fade is not vestibular-triggering, a slide is.

**Motion budget:** no more than one animated element per user action. A modal opening does not also animate its backdrop *and* its fields *and* a toast. Anything over 200 ms is a bug, not a flourish.

---

## 3. Component inventory

### 3.0 The four rules every component follows

1. **`forwardRef` always.** Radix, react-hook-form and focus management all need the DOM node.
2. **`className` is merged, never replaced.** Use `cn()` (clsx + `tailwind-merge`) so a caller's `className="w-64"` wins over the base.
3. **Focus is visible and consistent** — one ring style, everywhere: `focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2`. Never `outline-none` without a replacement.
4. **Every async state has a shape**: `idle → loading → success | error`, and each shape is rendered with text, not just a colour or a spinner.

Shared base and states, declared once:

```ts
// web/src/components/ui/cn.ts
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';
export const cn = (...i: ClassValue[]) => twMerge(clsx(i));

// The one focus ring. Referenced by every interactive component.
export const focusRing =
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ' +
  'focus-visible:ring-offset-2 focus-visible:ring-offset-surface';

// The one disabled treatment. Always accompanied by explanatory text (§8).
export const disabled =
  'disabled:pointer-events-none disabled:bg-grey-100 disabled:text-grey-400 ' +
  'disabled:border-grey-200';
```

**Universal state contract.** Every interactive component implements this table. Deviations are bugs.

| State | Trigger | Visual | Never |
|---|---|---|---|
| default | — | `border-border-strong`, `bg-surface`, `text-text` | — |
| hover | pointer over | `bg-grey-50`; primary darkens one ramp step | lighten a fill (§2.3) |
| focus-visible | keyboard focus | 2px `ring-primary` + 2px offset | remove outline without replacement |
| active | mouse down | `bg-grey-100`; primary → `primary-active` | shift layout |
| disabled | `disabled` / `aria-disabled` | `bg-grey-100`, `text-grey-400`, `cursor-not-allowed` | rely on colour alone |
| loading | `isPending` | inline 14px spinner replacing icon + **label text stays** | replace label with a spinner |
| error | `aria-invalid` | `border-danger` + `ring-danger/30` + message below | colour alone |

### 3.1 Form controls

| Component | Purpose | Variants | Notes |
|---|---|---|---|
| `Button` | Every action | `primary`, `secondary`, `ghost`, `danger` × `sm` 32px, `md` 36px, `lg` 44px | One primary per screen |
| `Input` | Single-line text | `default`, `error` × `sm`/`md` | `text-md` (16px) prevents iOS zoom |
| `Textarea` | Clinical notes | `default`, `error`, `auto-grow` | `leading-relaxed`; 4 rows default |
| `Select` | ≤ 15 known options | `default`, `error` | Radix `select` 2.4.1 |
| `Combobox` | Long or searchable lists | `single`, `multi`, `async` | Patients, drugs, ICD-10 |
| `Checkbox` | Independent booleans | `default`, `indeterminate` | — |
| `Radio` | 2–5 mutually exclusive | `default`, `card` | `card` = large tappable tiles |
| `DatePicker` | Dates and date-times | `date`, `datetime`, `range` | Never free-text dates |
| `FormField` | label + hint + error wrapper | `default`, `required`, `horizontal` | Owns the ARIA wiring |

```ts
// components/ui/button.tsx
import { cva, type VariantProps } from 'class-variance-authority';
export const button = cva(
  'inline-flex items-center justify-center gap-2 rounded-sm font-medium whitespace-nowrap ' +
  'transition-colors duration-100 ease-standard select-none ' + focusRing + disabled,
  {
    variants: {
      variant: {
        primary:   'bg-primary text-white hover:bg-primary-hover active:bg-primary-active',
        secondary: 'border border-border-strong bg-surface text-text hover:bg-grey-50',
        ghost:     'text-text-subtle hover:bg-grey-100 hover:text-text',
        danger:    'bg-danger text-white hover:bg-danger-fg',
      },
      size: {
        sm: 'h-8 px-3 text-sm',
        md: 'h-9 px-4 text-base',
        lg: 'h-11 px-5 text-md', // 44px - coarse pointers and the payment screen
      },
      block: { true: 'w-full', false: '' },
    },
    defaultVariants: { variant: 'secondary', size: 'md', block: false },
  },
);
export type ButtonProps = VariantProps<typeof button>;
```

```ts
// components/ui/input.tsx
export const input = cva(
  'w-full rounded-sm bg-surface text-text placeholder:text-grey-400 ' +
  'border transition-colors duration-100 ease-standard ' + focusRing + disabled +
  ' aria-[invalid=true]:border-danger aria-[invalid=true]:ring-danger/30',
  {
    variants: {
      size: { sm: 'h-8 px-2 text-base', md: 'h-9 px-3 text-md' },
      numeric: { true: 'tnum text-right', false: '' },
    },
    defaultVariants: { size: 'md', numeric: false },
  },
);
```

```ts
// components/ui/textarea.tsx
export const textarea = cva(
  'w-full rounded-sm border border-border-strong bg-surface px-3 py-2 text-md ' +
  'leading-relaxed text-text placeholder:text-grey-400 resize-y ' +
  'transition-colors duration-100 ease-standard ' + focusRing + disabled +
  ' aria-[invalid=true]:border-danger',
  { variants: { autoGrow: { true: 'resize-none field-sizing-content', false: '' } },
    defaultVariants: { autoGrow: false } },
);
```

```ts
// components/ui/select.tsx  (Radix Select - short, closed lists only)
export const selectTrigger = cva(
  'flex h-9 w-full items-center justify-between gap-2 rounded-sm border ' +
  'border-border-strong bg-surface px-3 text-md text-text ' +
  'data-[placeholder]:text-grey-400 ' + focusRing + disabled,
  { variants: { size: { sm: 'h-8 text-base', md: 'h-9' } }, defaultVariants: { size: 'md' } },
);
export const selectContent = cva(
  'z-dropdown overflow-hidden rounded-md border border-border bg-surface shadow-md ' +
  'data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 ' +
  'data-[state=closed]:animate-out data-[state=closed]:fade-out-0 motion-reduce:animate-none',
);
export const selectItem = cva(
  'relative flex cursor-default select-none items-center rounded-xs py-1.5 pl-8 pr-2 text-base ' +
  'outline-none data-[highlighted]:bg-grey-100 data-[state=checked]:font-medium ' +
  'data-[disabled]:pointer-events-none data-[disabled]:text-grey-400',
);
```

```ts
// components/ui/combobox.tsx  (searchable; patients, drugs, ICD-10, tariff items)
export const comboboxInput = cva(
  'h-9 w-full rounded-sm border border-border-strong bg-surface pl-9 pr-3 text-md ' +
  'placeholder:text-grey-400 ' + focusRing,
);
export const comboboxList = cva(
  'z-dropdown max-h-72 overflow-y-auto rounded-md border border-border bg-surface py-1 shadow-md',
);
export const comboboxOption = cva(
  'flex cursor-default items-center justify-between gap-3 px-3 py-2 text-base ' +
  'data-[active=true]:bg-primary-soft data-[active=true]:text-primary-fg ' +
  'data-[disabled]:text-grey-400',
);
// Key behaviour: type-ahead filters on patient_number + surname + phone;
// results are capped at 50 with a "%n more - keep typing" footer row.
// Selecting a patient dispatches onSelect(id) and refocuses the NEXT field,
// never the input - the clerk is done with this field.
```

```ts
// components/ui/checkbox.tsx
export const checkbox = cva(
  'peer size-4 shrink-0 rounded-xs border border-border-strong bg-surface ' +
  'transition-colors duration-100 ease-standard ' + focusRing +
  ' data-[state=checked]:border-primary data-[state=checked]:bg-primary ' +
  'data-[state=checked]:text-white data-[state=indeterminate]:bg-primary ' + disabled,
);
export const checkboxLabel = cva(
  'text-base text-text select-none peer-disabled:text-grey-400',
);
```

```ts
// components/ui/radio.tsx
export const radio = cva(
  'size-4 shrink-0 rounded-full border border-border-strong bg-surface ' + focusRing +
  ' data-[state=checked]:border-[5px] data-[state=checked]:border-primary ' + disabled,
);
export const radioCard = cva(
  'flex cursor-pointer items-start gap-3 rounded-md border p-4 transition-colors ' +
  'duration-100 ease-standard hover:bg-grey-50 ' + focusRing,
  { variants: { selected: {
      true:  'border-primary bg-primary-soft',
      false: 'border-border-strong bg-surface' } },
    defaultVariants: { selected: false } },
);
```

```ts
// components/ui/date-picker.tsx  (Radix Popover + react-day-picker)
export const dateTrigger = cva(
  'flex h-9 w-full items-center gap-2 rounded-sm border border-border-strong bg-surface ' +
  'px-3 text-left text-md tnum ' + focusRing + disabled,
  { variants: { empty: { true: 'text-grey-400', false: 'text-text' } },
    defaultVariants: { empty: true } },
);
export const dateCalendar = cva(
  'z-dropdown rounded-md border border-border bg-surface p-3 shadow-md ' +
  'transition-opacity duration-150 ease-standard motion-reduce:transition-none',
);
// Rules: no free-text date entry anywhere (a mistyped DOB is a clinical hazard).
// "Today" and "Now" are always one click. Future dates are disabled for
// date-of-birth. The trigger shows ISO-ish "14 Feb 2026, 09:41" via date-fns.
```

```ts
// components/ui/form-field.tsx
export const formField = cva('flex flex-col', {
  variants: {
    layout: { vertical: 'gap-field', horizontal: 'sm:grid sm:grid-cols-[10rem_1fr] sm:gap-4' },
  },
  defaultVariants: { layout: 'vertical' },
});
export const formLabel = cva(
  'text-base font-medium text-text ' +
  'data-[required=true]:after:ml-0.5 data-[required=true]:after:text-danger ' +
  'data-[required=true]:after:content-["*"]',
);
export const formHint  = cva('text-sm text-text-muted');
export const formError = cva('flex items-start gap-1.5 text-sm font-medium text-danger');
// FormField generates one id via useId() and wires, in this exact order:
//   <label htmlFor={id}>            -> label
//   aria-describedby={hintId errId} -> hint AND error, hint first
//   aria-invalid={!!error}          -> error state
//   <p id={hintId} class=formHint>  -> static guidance
//   <p id={errId} role="alert">     -> error, announced immediately
// If a screen has more than one error after submit, ALSO render one
// role="alert" summary above the form: "3 fields need attention".
```

### 3.2 Display and data

| Component | Purpose | Variants / states |
|---|---|---|
| `Card` | Grouped content block | `default`, `raised`, `flat`, `interactive` |
| `StatCard` | One headline number | `default`, `primary`, `warning`, `danger`; loading skeleton |
| `DataTable` | All tabular data | sortable, sticky header, selectable rows, empty, loading, error |
| `Badge` | Small status label | `neutral`, `primary`, `success`, `warning`, `danger`, `info` |
| `Avatar` | Person identity | `xs` 20, `sm` 28, `md` 36, `lg` 48; `initials`, `image` |
| `EmptyState` | Zero-result explanation | `no-data`, `no-results`, `no-permission`, `offline-unavailable` |
| `Skeleton` | Loading placeholder | `text`, `block`, `circle`, `row` |
| `ProgressBar` | Determinate/indeterminate | `default`, `success`, `warning` |
| `Alert` | Inline message | `info`, `success`, `warning`, `danger` |
| `Timeline` | Encounter history | `default`, `compact`; per-event icon + sync badge |
| `Tabs` | Content sections | `underline` (in-page), `pill` (filters) |
| `Pagination` | Large result sets | `numbered`, `simple` |

```ts
// components/ui/card.tsx
export const card = cva('rounded-md bg-surface', {
  variants: {
    variant: {
      default:     'border border-border shadow-sm',
      raised:      'border border-border shadow-md',
      flat:        'border border-border',
      interactive: 'border border-border shadow-sm transition-shadow duration-150 ' +
                   'ease-standard hover:shadow-md focus-within:ring-2 ' +
                   'focus-within:ring-ring focus-within:ring-offset-2',
    },
    padding: { none: 'p-0', sm: 'p-3', md: 'p-4', lg: 'p-6' },
  },
  defaultVariants: { variant: 'default', padding: 'md' },
});
export const cardHeader = cva(
  'flex items-center justify-between gap-4 border-b border-border px-4 py-3',
);
export const cardTitle = cva('text-lg font-medium text-text');
```

```ts
// components/ui/stat-card.tsx
export const statCard = cva('flex flex-col gap-1 rounded-md border p-4', {
  variants: {
    tone: {
      default: 'border-border bg-surface',
      primary: 'border-primary-200 bg-primary-50',
      warning: 'border-warning-soft bg-warning-soft',
      danger:  'border-danger-soft bg-danger-soft',
    },
  },
  defaultVariants: { tone: 'default' },
});
export const statValue = cva(
  'tnum text-3xl font-semibold leading-tight text-text', // always tabular
);
export const statLabel = cva(
  'text-xs font-semibold uppercase tracking-wide text-text-muted',
);
export const statDelta = cva('text-sm tnum', {
  variants: { direction: { up: 'text-success', down: 'text-danger', flat: 'text-text-muted' } },
});
// A StatCard whose value is not yet known renders <Skeleton class="h-8 w-16" />,
// never "0" - a false zero in a queue count sends staff to the wrong room.
```

```ts
// components/ui/data-table.tsx
export const tableWrap = cva('relative w-full overflow-auto rounded-md border border-border bg-surface');
export const table = cva('w-full caption-bottom border-collapse text-sm');
export const thead = cva(
  'sticky top-0 z-sticky bg-grey-100 text-xs font-semibold uppercase tracking-wide text-text-subtle',
);
export const th = cva(
  'h-9 whitespace-nowrap border-b border-border px-3 text-left align-middle',
  { variants: { align: { left: 'text-left', right: 'text-right tnum', center: 'text-center' } },
    defaultVariants: { align: 'left' } },
);
export const td = cva('h-11 border-b border-border px-3 align-middle text-text', {
  variants: { align: { left: '', right: 'tnum text-right', center: 'text-center' } },
  defaultVariants: { align: 'left' },
});
export const tr = cva(
  'transition-colors duration-100 ease-standard hover:bg-grey-50 ' +
  'data-[selected=true]:bg-primary-soft',
);
export const sortButton = cva(
  'inline-flex items-center gap-1 rounded-xs uppercase hover:text-text ' + focusRing,
);
// Behaviour contract:
//  sticky header inside a max-h-[70vh] scroll container;
//  clicking a sortable th toggles asc -> desc -> unsorted, announced via
//    aria-sort on the th, not by moving an arrow only;
//  row count > 50 -> virtualise (TanStack Virtual) rather than paginate;
//  row count 0 -> <EmptyState>, NEVER an empty <tbody>;
//  loading -> 8 <Skeleton variant="row">, keeping the real column widths;
//  every row's last cell is the SyncBadge (§4.2).
```

```ts
// components/ui/badge.tsx
export const badge = cva(
  'inline-flex items-center gap-1 rounded-xs px-2 py-0.5 text-xs font-medium whitespace-nowrap',
  {
    variants: {
      tone: {
        neutral: 'bg-grey-100 text-text-subtle',
        primary: 'bg-primary-soft text-primary-fg',
        success: 'bg-success-soft text-success-fg',
        warning: 'bg-warning-soft text-warning-fg',
        danger:  'bg-danger-soft text-danger-fg',
        info:    'bg-info-soft text-info-fg',
      },
    },
    defaultVariants: { tone: 'neutral' },
  },
);
// EVERY badge carries an icon or a short word. There is no colour-only badge
// in this codebase - see 03-UI-PATTERNS.md section 8 and section 10.
```

```ts
// components/ui/avatar.tsx
export const avatar = cva(
  'inline-flex shrink-0 items-center justify-center overflow-hidden rounded-full ' +
  'bg-grey-100 font-medium text-text-subtle select-none',
  {
    variants: {
      size: { xs: 'size-5 text-[10px]', sm: 'size-7 text-xs',
              md: 'size-9 text-sm',    lg: 'size-12 text-base' },
    },
    defaultVariants: { size: 'md' },
  },
);
// Initials from surname + first name. avatars are NEVER the only identifier:
// always render "AMOAH, Kofi" beside them, because two patients share initials.
```

```ts
// components/ui/empty-state.tsx
export const emptyState = cva(
  'flex flex-col items-center justify-center gap-3 rounded-md border border-dashed ' +
  'border-border-strong bg-surface px-6 py-12 text-center',
);
export const emptyIcon = cva('text-grey-400'); // lucide icon, 32px
export const emptyTitle = cva('text-lg font-medium text-text');
export const emptyBody  = cva('max-w-sm text-base text-text-muted');
// Four presets, each with DIFFERENT copy and a different suggested action:
//  no-data       "No patients registered yet"        -> [Register patient]
//  no-results    "No patient matches 'kofi amo'"    -> [Clear filters]
//  no-permission "Your role cannot view billing"     -> no action, explains why
//  offline-unavailable "Referral lookup needs a connection" -> explains + [Retry]
```

```ts
// components/ui/skeleton.tsx
export const skeleton = cva(
  'animate-pulse rounded-xs bg-grey-100 motion-reduce:animate-none',
  { variants: {
      variant: {
        text:   'h-4 w-full',
        block:  'h-24 w-full',
        circle: 'size-9 rounded-full',
        row:    'h-11 w-full', // matches DataTable row height exactly
      } },
    defaultVariants: { variant: 'text' } },
);
```

```ts
// components/ui/progress-bar.tsx
export const progressTrack = cva('h-1.5 w-full overflow-hidden rounded-full bg-grey-200');
export const progressFill = cva('h-full rounded-full transition-[width] duration-200 ease-standard', {
  variants: {
    tone: { default: 'bg-primary', success: 'bg-success', warning: 'bg-warning' },
    indeterminate: { true: 'w-1/3 animate-[indeterminate_1.2s_linear_infinite]', false: '' },
  },
  defaultVariants: { tone: 'default', indeterminate: false },
});
// Determinate progress always renders "42 of 118" beside it. An indeterminate
// bar always renders a text label too ("Checking for updates").
```

```ts
// components/ui/alert.tsx
export const alert = cva('flex items-start gap-3 rounded-md border p-3 text-base', {
  variants: {
    tone: {
      info:    'border-info-soft bg-info-soft text-info-fg',
      success: 'border-success-soft bg-success-soft text-success-fg',
      warning: 'border-warning-soft bg-warning-soft text-warning-fg',
      danger:  'border-danger-soft bg-danger-soft text-danger-fg',
    },
  },
  defaultVariants: { tone: 'info' },
});
// Every Alert carries a lucide icon, a bold one-line title and a body. The
// danger tone also sets role="alert"; the others use role="status".
```

```ts
// components/ui/timeline.tsx
export const timeline     = cva('relative flex flex-col gap-0 pl-6');
export const timelineRail = cva('absolute left-[9px] top-2 bottom-2 w-px bg-border');
export const timelineItem = cva('relative flex gap-3 py-3');
export const timelineDot  = cva(
  'absolute -left-6 top-4 size-[18px] rounded-full border-2 border-surface',
  { variants: { tone: { neutral: 'bg-grey-300', primary: 'bg-primary',
                        success: 'bg-success', warning: 'bg-warning', danger: 'bg-danger' } },
    defaultVariants: { tone: 'neutral' } },
);
// The encounter history: registration -> vitals -> triage -> consultation ->
// lab -> pharmacy -> payment -> closed. Each row shows time (tnum), actor,
// a one-line summary, and a SyncBadge. Newest first, with "Load earlier".
```

### 3.3 Overlays and navigation

| Component | Purpose | Variants |
|---|---|---|
| `Dialog` | Blocking decision / short form | `sm` 400, `md` 560, `lg` 720, `xl` 960 |
| `Drawer` | Side panel, non-modal friendly | `right` 480, `right-wide` 640, `bottom` (small screens) |
| `Toast` | Transient confirmation | `success`, `error`, `warning`, `info`, `loading` |
| `Tooltip` | 1-line explanation | `top`, `right`, `bottom`, `left`; 400 ms delay |
| `DropdownMenu` | Contextual actions | `default`, `with-icons`, `with-dividers` |
| `CommandPalette` | Ctrl+K global search | `patients`, `actions`, `navigation` |
| `Pagination` | Result paging | `numbered`, `simple` |
| `Breadcrumbs` | Location in hierarchy | `default`, `collapsed` |
| `Stepper` | Multi-step wizard | `horizontal`, `vertical`, `compact` |
| `PrinterFrame` | Print preview + print trigger | `receipt-58`, `receipt-80`, `a4` |

```ts
// components/ui/dialog.tsx  (Radix Dialog 1.3.1)
export const dialogOverlay = cva(
  'fixed inset-0 z-modal-backdrop bg-grey-950/40 ' +
  'data-[state=open]:animate-in data-[state=open]:fade-in-0 ' +
  'data-[state=closed]:animate-out data-[state=closed]:fade-out-0 ' +
  'motion-reduce:animate-none',
);
export const dialogContent = cva(
  'fixed left-1/2 top-1/2 z-modal w-full -translate-x-1/2 -translate-y-1/2 rounded-lg ' +
  'border border-border bg-surface p-6 shadow-lg ' +
  'data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 ' +
  'data-[state=closed]:animate-out data-[state=closed]:fade-out-0 motion-reduce:animate-none',
  { variants: { size: { sm: 'max-w-[400px]', md: 'max-w-[560px]',
                        lg: 'max-w-[720px]', xl: 'max-w-[960px]' } },
    defaultVariants: { size: 'md' } },
);
export const dialogFooter = cva('mt-6 flex justify-end gap-2');
// Contract: focus moves to the first field (or the primary button if there is
// no field); Escape closes; the primary action is last in DOM order; while
// `isPending` the dialog stays open, the primary button shows its spinner
// inline and BOTH buttons become disabled. A dialog never closes on success
// before the record is committed to Dexie.
```

```ts
// components/ui/drawer.tsx
export const drawerContent = cva(
  'fixed z-modal bg-surface shadow-lg ' +
  'data-[state=open]:animate-in data-[state=closed]:animate-out motion-reduce:animate-none',
  {
    variants: {
      side: {
        right:      'inset-y-0 right-0 w-[480px] border-l border-border ' +
                    'data-[state=open]:slide-in-from-right data-[state=closed]:slide-out-to-right',
        'right-wide': 'inset-y-0 right-0 w-[640px] border-l border-border ' +
                    'data-[state=open]:slide-in-from-right data-[state=closed]:slide-out-to-right',
        bottom:     'inset-x-0 bottom-0 max-h-[85vh] rounded-t-lg border-t border-border ' +
                    'data-[state=open]:slide-in-from-bottom data-[state=closed]:slide-out-to-bottom',
      },
    },
    defaultVariants: { side: 'right' },
  },
);
```

```ts
// components/ui/toast.tsx  (sonner 2.0.8 - mounted once in the app shell)
export const toaster = cva('toaster group', {
  variants: { position: { 'bottom-right': '', 'top-center': '' } },
  defaultVariants: { position: 'bottom-right' },
});
export const toastSurface = cva(
  'group flex items-center gap-3 rounded-md border border-border bg-surface p-3 shadow-md ' +
  'text-base text-text',
  { variants: { tone: {
      success: 'border-l-2 border-l-success',
      error:   'border-l-2 border-l-danger',
      warning: 'border-l-2 border-l-warning',
      info:    'border-l-2 border-l-info',
      loading: 'border-l-2 border-l-primary' } },
    defaultVariants: { tone: 'info' } },
);
// Rules: duration 4000ms for success/info, 8000ms for warning, sticky (0 =
// manual dismiss) for error. Errors always carry a "Dismiss" and, where a
// retry is possible, a "Retry" action. Maximum 3 stacked; a 4th collapses
// into "+2 more". Toasts NEVER carry the only copy of important information.
```

```ts
// components/ui/tooltip.tsx
export const tooltipContent = cva(
  'z-tooltip max-w-xs rounded-sm bg-grey-900 px-2 py-1 text-xs text-white shadow-md ' +
  'data-[state=delayed-open]:animate-in data-[state=delayed-open]:fade-in-0 ' +
  'motion-reduce:animate-none',
);
// delayDuration={400} skipDelayDuration={300}. Tooltips never contain
// information a user needs in order to complete a task - they are reminders.
```

```ts
// components/ui/dropdown-menu.tsx
export const menuContent = cva(
  'z-dropdown min-w-[11rem] overflow-hidden rounded-md border border-border bg-surface ' +
  'p-1 shadow-md data-[state=open]:animate-in data-[state=open]:fade-in-0 ' +
  'data-[state=open]:zoom-in-95 motion-reduce:animate-none',
);
export const menuItem = cva(
  'flex cursor-default select-none items-center gap-2 rounded-xs px-2 py-1.5 text-base ' +
  'text-text outline-none data-[highlighted]:bg-grey-100 ' +
  'data-[disabled]:pointer-events-none data-[disabled]:text-grey-400',
);
export const menuItemDanger = cva(
  'flex cursor-default select-none items-center gap-2 rounded-xs px-2 py-1.5 text-base ' +
  'text-danger outline-none data-[highlighted]:bg-danger-soft',
);
```

```ts
// components/ui/command-palette.tsx  (Ctrl+K / Cmd+K)
export const paletteContent = cva(
  'fixed left-1/2 top-[15%] z-modal w-full max-w-[560px] -translate-x-1/2 overflow-hidden ' +
  'rounded-lg border border-border bg-surface shadow-lg ' +
  'data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 ' +
  'motion-reduce:animate-none',
);
export const paletteInput = cva(
  'h-12 w-full border-b border-border bg-transparent px-4 text-md outline-none ' +
  'placeholder:text-grey-400',
);
export const paletteGroup = cva(
  'px-2 py-1 text-xs font-semibold uppercase tracking-wide text-text-muted',
);
export const paletteItem = cva(
  'flex items-center gap-3 px-3 py-2 text-base data-[selected=true]:bg-primary-soft ' +
  'data-[selected=true]:text-primary-fg',
);
// Groups, in order: Patients (number, surname, phone - searches Dexie via
// useLiveQuery, so it works offline), Actions ("Register patient", "New
// encounter"), Navigation. Up to 8 per group. Enter runs the highlighted item.
// This is the fastest path to a patient record and should be advertised in the
// top bar placeholder: "Search patients   Ctrl K".
```

```ts
// components/ui/breadcrumbs.tsx
export const breadcrumbList = cva('flex items-center gap-1 text-sm text-text-muted');
export const breadcrumbLink = cva('rounded-xs px-1 hover:text-text ' + focusRing);
export const breadcrumbCurrent = cva('px-1 font-medium text-text');
export const breadcrumbSep = cva('text-grey-400'); // lucide ChevronRight, 14px
// Maximum three levels: Queue / Amoah, Kofi / Consultation. Beyond three,
// collapse the middle into a DropdownMenu rather than wrapping to a second line.
```

```ts
// components/ui/pagination.tsx
export const paginationRoot = cva('flex items-center justify-between gap-4 py-3');
export const paginationPage = cva(
  'tnum inline-flex h-8 min-w-8 items-center justify-center rounded-sm px-2 text-base ' +
  'transition-colors duration-100 ease-standard ' + focusRing,
  { variants: { active: { true: 'bg-primary text-white', false: 'text-text hover:bg-grey-100' } },
    defaultVariants: { active: false } },
);
// Always renders "Showing 1-25 of 340" on the left (tnum) and the controls on
// the right. Never renders when there is one page.
```

```ts
// components/ui/stepper.tsx
export const stepperRoot = cva('flex items-center gap-0');
export const stepperStep = cva('flex items-center gap-2', {
  variants: { state: {
      complete: 'text-primary', current: 'text-text font-medium',
      upcoming: 'text-text-muted', error: 'text-danger' } },
  defaultVariants: { state: 'upcoming' } },
);
export const stepperIndex = cva(
  'tnum flex size-6 items-center justify-center rounded-full border text-xs',
  { variants: { state: {
      complete: 'border-primary bg-primary text-white',
      current:  'border-primary bg-primary-soft text-primary-fg',
      upcoming: 'border-border-strong bg-surface text-text-muted',
      error:    'border-danger bg-danger-soft text-danger-fg' } },
    defaultVariants: { state: 'upcoming' } },
);
// Used by registration (Patient -> Household -> Confirm) and admission
// (Ward -> Bed -> Tariff -> Confirm). Steps are clickable when revisiting an
// earlier step is safe; not clickable in a clinical order that must be linear.
```

```ts
// components/ui/tabs.tsx
export const tabsList = cva('flex items-center gap-1 border-b border-border');
export const tabsTrigger = cva(
  'relative -mb-px px-3 py-2 text-base text-text-muted transition-colors duration-100 ' +
  'ease-standard hover:text-text ' + focusRing +
  ' data-[state=active]:font-medium data-[state=active]:text-text ' +
  ' data-[state=active]:after:absolute data-[state=active]:after:inset-x-0 ' +
  ' data-[state=active]:after:-bottom-px data-[state=active]:after:h-0.5 ' +
  ' data-[state=active]:after:bg-primary',
);
export const tabPanel = cva(
  'pt-4 data-[state=active]:animate-in data-[state=active]:fade-in-0 ' +
  'motion-reduce:animate-none',
);
// No sliding indicator animation between tabs: the panel content is already
// changing, and sliding a 2px underline adds 200ms of nothing.
```

```ts
// components/ui/printer-frame.tsx
export const printerFrame = cva(
  'mx-auto bg-white p-4 font-mono text-[11px] leading-[1.35] text-black shadow-md',
  { variants: { paper: { 'receipt-58': 'w-[48mm]',   // 32 columns at 12 cpi
                         'receipt-80': 'w-[72mm]',   // 42 columns (safe) up to 48
                         a4: 'w-[210mm] p-0 shadow-none' } },
    defaultVariants: { paper: 'receipt-58' } },
);
export const printerToolbar = cva(
  'mb-4 flex items-center justify-between gap-3 rounded-md border border-border ' +
  'bg-surface p-3 print:hidden',
);
// The print preview shell. Both the preview and the print stylesheet consume
// the SAME composed receipt component, so what the cashier sees is what the
// thermal head prints. The preview never uses a screenshot image.
// Before printing, run the preflight in section 7.4: printer selected, paper
// width, darkness, and a QR readability reminder for referral slips.
```

---

## 4. The sync / offline status pattern

This is the most novel UI surface in the app and the one an examiner will ask about. It exists because of `00-ARCHITECTURE.md` §4.4: a write is committed to Dexie (step 3) and the UI updates (step 4) **before** the network is involved (step 5). Something must tell the user that step 5 has not happened yet.

### 4.1 `SyncStatusIndicator` — the top bar, clinic-wide truth

Exactly five states. There is no sixth, and `offline` is deliberately not red.

```ts
// web/src/sync/types.ts
export type SyncState = 'synced' | 'syncing' | 'pending' | 'error' | 'offline';

export interface SyncStatus {
  state: SyncState;
  pendingCount: number;        // operations in the outbox
  inFlightCount: number;       // operations pushed in the current batch
  batchTotal: number;          // size of the batch being pushed
  lastSyncedAt: string | null; // ISO UTC (00-ARCHITECTURE.md 7.3)
  lastError: { code: string; message: string; at: string } | null;
  offlineSince: string | null; // set on the transition to 'offline'
}
// Held in the Zustand sync store (00-ARCHITECTURE.md 4.3). Written ONLY by the
// sync engine. Components read it; components never set it.
```

| State | Condition | Icon (lucide) | Colour | Label shown |
|---|---|---|---|---|
| `synced` | online, outbox empty | `CloudCheck` | `text-success-fg` on `success-soft` | "All records synced" |
| `syncing` | a push/pull is in flight | `RefreshCw` (spin, 1000 ms linear) | `text-primary-fg` on `primary-soft` | "Syncing 12 of 40…" + `ProgressBar` |
| `pending` | online, N > 0 queued | `CloudUpload` | `text-warning-fg` on `warning-soft` | "12 waiting to sync" |
| `error` | last push rejected or retry budget exhausted | `CloudAlert` | `text-danger-fg` on `danger-soft` | "Sync problem — 12 waiting" |
| `offline` | `navigator.onLine === false` **or** last attempt failed with a network error | `CloudOff` | `text-warning-fg` on `warning-soft` | "Offline — 12 saved on this device" |

```ts
// web/src/sync/SyncStatusIndicator.tsx  (abridged)
const tone: Record<SyncState, string> = {
  synced:  'bg-success-soft text-success-fg',
  syncing: 'bg-primary-soft text-primary-fg',
  pending: 'bg-warning-soft text-warning-fg',
  offline: 'bg-warning-soft text-warning-fg', // NOT danger. Being offline is normal.
  error:   'bg-danger-soft text-danger-fg',
};
const Icon = { synced: CloudCheck, syncing: RefreshCw, pending: CloudUpload,
               offline: CloudOff, error: CloudAlert } as const;

export function SyncStatusIndicator() {
  const s = useSyncStore();                    // Zustand
  const label = useSyncLabel(s);               // pure function, table above
  return (
    <button
      type="button"
      onClick={() => setSyncPanelOpen(true)}
      className={cn('inline-flex h-8 items-center gap-2 rounded-sm px-2.5 text-sm',
                    tone[s.state], focusRing)}
    >
      <Icon[s.state] className={cn('size-4', s.state === 'syncing' && 'animate-spin motion-reduce:animate-none')} />
      <span>{label}</span>
      {s.state === 'syncing' && s.batchTotal > 0 && (
        <ProgressBar value={s.inFlightCount} max={s.batchTotal} className="w-16" />
      )}
    </button>
  );
}
// aria-live is on a SEPARATE visually-hidden node, not on this button, so a
// screen reader is not re-announced the whole control on every batch tick.
// <p aria-live="polite" aria-atomic="true" className="sr-only">{label}</p>
// Announce state TRANSITIONS only (synced->pending, pending->syncing,
// syncing->error). Do not announce progress percentages.
```

### 4.2 `SyncBadge` — one row's truth

Appears as the final cell of every `DataTable` row and on every detail header.

```ts
// components/ui/sync-badge.tsx
export const syncBadge = cva(
  'inline-flex items-center gap-1 rounded-xs px-1.5 py-0.5 text-xs font-medium whitespace-nowrap',
  {
    variants: {
      state: {
        pending:  'bg-warning-soft text-warning-fg',
        synced:   'bg-success-soft text-success-fg',
        conflict: 'bg-danger-soft text-danger-fg',
      },
    },
    required: true,
  },
);
export const syncBadgeIcon = { pending: Clock, synced: Check, conflict: TriangleAlert } as const;
export const syncBadgeLabel = { pending: 'Pending', synced: 'Synced', conflict: 'Conflict' } as const;
// Text + icon ALWAYS. Never a bare coloured dot: at 1 m, in poor light, a
// 6px amber dot and a 6px green dot are the same dot.
```

Conflict detection, retry policy and the merge rules themselves belong to `02-SYNC-PROTOCOL.md`. This document only specifies how the result surfaces.

### 4.3 The rule table — what the user is told and what they may do

| State | What the user is told | What they can still do | What is blocked | Where it appears |
|---|---|---|---|---|
| `synced` | "All records synced", with "just now" / "5 min ago" from `lastSyncedAt` | Everything | Nothing | Top bar |
| `syncing` | "Syncing 12 of 40…", determinate progress | Everything. Data entry is **never** paused or dimmed for a sync | Nothing | Top bar |
| `pending` (online) | "12 waiting to sync" | Everything | Nothing | Top bar |
| `pending` (offline) | "Offline — 12 saved on this device" | **All clinical work**: register, triage, consult, order labs, prescribe, dispense, take payment | Referral *lookup* from another facility, user management, reports that must be server-computed | Top bar + every row |
| `offline` | Same as above, plus "Last synced 14 Feb, 09:12" | All clinical work, indefinitely (until the grace window, §4.4) | Nothing clinical | Top bar |
| `error` | "Sync problem — 12 waiting" + a plain-language reason and a Retry action | All clinical work | Nothing clinical; only the outbox is stuck | Top bar (danger tone), plus a dismissible `Alert` on the dashboard |
| `conflict` (per row) | Row badge "Conflict" + a banner: "This patient was edited on another device" | Recording new clinical facts | Editing the conflicting demographic fields until resolved | The row and the patient detail header |

**The rule that overrides every other rule on this page:** *a user must never lose work, and must never be blocked from recording care, because of connectivity.* Concretely, this means:

- No modal, ever, that says "you are offline, try again". That is a dead end.
- The Save button is never disabled because of sync state. It is disabled only for validation errors or a pending write to Dexie.
- "Offline" never interrupts a form in progress. The indicator changes quietly; the caret does not move.
- The Pending badge is not an error and never uses danger colours.
- If the outbox grows past a threshold (say 500 operations, roughly a shift's work), show a **neutral** warning advising the user to connect and sync when convenient — not a block.

### 4.4 The offline grace period (configurable, default 7 days)

`00-ARCHITECTURE.md` §5.1 defines `OFFLINE_GRACE_PERIOD` (default `168h`) and `OFFLINE_POLICY` (`block_writes` by default, or `warn_only`). Whatever the configured window, it must be surfaced *early and calmly*, because the failure mode is a device that locks in the middle of a clinic day.

**Express every threshold as a percentage of the configured window, never as a hard-coded day count.** Write `useOfflineGrace()` to compute from the window it is given, so changing `OFFLINE_GRACE_PERIOD` needs no UI change.

| Elapsed offline | UI treatment |
|---|---|
| < 60% of window | Nothing beyond the normal `offline` indicator |
| 60 – 85% of window | Top bar indicator gains a suffix: "Offline · reconnect within N days". `info` `Alert` on the dashboard, dismissible for the session |
| 85 – 100% of window | `warning` `Alert`, **not** dismissible, on every screen's page header: "This device must connect to the internet within N days to keep working offline." Shows the exact deadline date and time in local time |
| Final day of window | `warning` `Alert` plus a persistent top-bar chip counting down in hours |
| > 100% of window | **Under `block_writes`:** online sign-in required. The app stays **readable** — all Dexie data remains viewable — but new writes are disabled, with a full-screen `Alert` explaining exactly why and what to do. Unlocks without a reload as soon as connectivity returns and re-auth succeeds. **Under `warn_only`:** writes continue; the banner becomes a persistent, non-dismissible reminder that the device is past its recommended re-authentication window |

```ts
// web/src/sync/useOfflineGrace.ts
export function useOfflineGrace(
  offlineSince: string | null,
  windowHours = 168,          // from GET /meta — never hard-coded here
  policy: 'block_writes' | 'warn_only' = 'block_writes',
) {
  const elapsedH = offlineSince
    ? differenceInHours(new Date(), new Date(offlineSince))
    : 0;
  const pct = elapsedH / windowHours;
  const hoursLeft = Math.max(0, windowHours - elapsedH);

  if (pct < 0.60) return { level: 'none' as const, hoursLeft, policy };
  if (pct < 0.85) return { level: 'info' as const, hoursLeft, policy };
  if (pct <= 1)   return { level: 'warning' as const, hoursLeft, policy };
  return { level: 'expired' as const, hoursLeft: 0, policy };
}
// The deadline comes from offlineSince, stored on the DEVICE, so it survives a
// reload and cannot be reset by clearing the session.
// A server-side revocation ALWAYS wins, regardless of policy — see
// 00-ARCHITECTURE.md §5.1, final paragraph.
```

Never use a `dialog` for the grace warning. It must never steal focus from a form mid-entry.

---

## 5. Layout patterns

### 5.1 App shell

```
┌────────────┬──────────────────────────────────────────────────────────────┐
│ SIDEBAR    │ TOP BAR  (h-14, sticky, z-sticky, border-b, bg-surface)      │
│ w-60       │  Riverside PHC · Kumasi      [SyncStatusIndicator] [🔍 Ctrl K]│
│ fixed      │                                        [Avatar ▾ user menu] │
│ bg-surface ├──────────────────────────────────────────────────────────────┤
│ border-r   │ PAGE HEADER  (title, breadcrumb, primary action)             │
│            ├──────────────────────────────────────────────────────────────┤
│ ▸ Dashboard│                                                              │
│ ▸ Queue    │ MAIN  (bg-canvas, p-gutter, max-w-[1600px] mx-auto)         │
│ ▸ Patients │                                                              │
│ ▸ Lab      │                                                              │
│ ▸ Pharmacy │                                                              │
│ ▸ Billing  │                                                              │
│ ▸ Admin    │                                                              │
│            │                                                              │
│ [collapse] │                                                              │
└────────────┴──────────────────────────────────────────────────────────────┘
```

- **Sidebar:** `fixed inset-y-0 left-0 w-60 bg-surface border-r border-border`, collapsible to `w-14` (icons only, `Tooltip` on hover, state in Zustand and persisted). Nav items are **filtered by role** from the §6 role table in `00-ARCHITECTURE.md` — a `cashier` never sees "Lab worklist". Active item: `bg-primary-soft text-primary-fg font-medium` with a 2px `border-l-primary` marker. Hiding a link is usability; the service layer is the security boundary.
- **Top bar:** sticky, `h-14`, `bg-surface/95 backdrop-blur-sm` only if it measurably helps — otherwise plain `bg-surface` (backdrop blur is a per-frame GPU cost on the laptops this runs on).
- **Main:** `bg-canvas`, `p-gutter`, content capped at `max-w-[1600px]` so the queue board does not stretch to a 34-inch monitor.
- **Density toggle:** the shell exposes `compact` and `comfortable` (row height 40px vs 44px), persisted per device. A cashier on a 1366×768 laptop needs compact; a nurse on a ward tablet needs comfortable.

### 5.2 Page header

```tsx
<header className="sticky top-0 z-sticky -mx-gutter border-b border-border bg-surface px-gutter py-3">
  <Breadcrumbs items={[{ label: 'Queue', to: '/queue' }, { label: 'Amoah, Kofi' }]} />
  <div className="mt-1 flex items-center justify-between gap-4">
    <div className="flex items-baseline gap-3">
      <h1 className="text-2xl font-semibold text-text">Consultation</h1>
      <Badge tone="primary">PT-000431</Badge>
      <SyncBadge state={encounter.syncState} />
    </div>
    <div className="flex items-center gap-2">
      <Button variant="ghost">Cancel</Button>
      <Button variant="primary">Save &amp; close encounter</Button>
    </div>
  </div>
</header>
```

Rules: one `h1` per page; the identity badge (patient/visit number) always sits next to the title so a mis-identified patient is caught at a glance; exactly one primary button; the header is sticky so the primary action is reachable from anywhere on a long form.

### 5.3 List / detail

- **List** = `DataTable` inside a `Card`. Filters in a toolbar row above it. Row click navigates to detail; the row's trailing `…` `DropdownMenu` holds secondary actions (print, cancel, reassign) and stops propagation.
- **Detail** = `Card` sections stacked in a single column, `max-w-[880px]`. The synopsis (name, age, sex, patient number, allergies, active encounter stage) is a `Card` pinned at the top.
- **Return path:** detail pages always render breadcrumbs back to the list, and the list restores scroll position and filter state from the URL (`TanStack Router` search params), so a clerk can bounce between the queue and a patient fifty times a shift without losing their place.
- List state lives in the URL, not in component state. That is what makes the back button behave.

### 5.4 Form

```
   single column, max-w-[640px]                 ┌─ sticky action bar (bottom) ─┐
   ┌──────────────────────────────────┐         │ Cancel   [ Save patient ]    │
   │ PATIENT DETAILS   (fieldset)     │         └──────────────────────────────┘
   │  Surname *         [___________] │
   │  First name *      [___________] │
   │  Sex *             ( ) M  ( ) F  │
   ├──────────────────────────────────┤
   │ CONTACT           (fieldset)     │
   │  Phone             [___________] │
   └──────────────────────────────────┘
```

- **One column, `max-w-[640px]`.** Two-column clinical forms cause skipped fields; the eye returns to the wrong row after an interruption. The only exception is a pair of genuinely short, related fields (sex + date of birth) inside a `grid-cols-2` with `gap-4`.
- **Grouped `fieldset`s** with a `legend` styled `text-xs uppercase tracking-wide text-text-muted`, separated by `space-y-8`.
- **Sticky action bar:** `sticky bottom-0 z-sticky -mx-6 border-t border-border bg-surface px-6 py-3`, holding Cancel (ghost) and one primary. Because the bar is sticky, the primary action is always on screen — this is what lets the form be long without a wizard.
- **Validation timing:** validate on blur for a field the user has already filled, and on submit for everything. Never validate on keystroke 1 of a phone number.
- **Submit:** Enter in any single-line input submits. `Ctrl+Enter` submits from a `Textarea`. Escape cancels — with a confirm `Dialog` only if the form is dirty.
- **Draft safety:** a dirty form persists to a Zustand draft slice on a 2 s debounce, so an accidental navigation or reload does not lose a half-entered patient. A small "Draft restored" `Alert` appears on return.

### 5.5 The queue board (workstation pattern)

The queue board is the operational heart of the app. It shows encounters as they move through the state machine in `00-ARCHITECTURE.md` §7.5.

```
┌─ Waiting vitals ─┬─ Awaiting doctor ─┬─ Awaiting lab ─┬─ Awaiting payment ─┬─ Awaiting pharmacy ─┐
│ 4  ⏱ 12m        │ 7  ⏱ 31m ⚠       │ 3  ⏱ 18m      │ 2  ⏱ 6m           │ 5  ⏱ 9m             │
├─────────────────┼───────────────────┼────────────────┼───────────────────┼─────────────────────┤
│ ┌─────────────┐ │ ┌───────────────┐ │ ┌────────────┐ │ ┌───────────────┐ │ ┌─────────────────┐ │
│ │AMOAH, Kofi  │ │ │MENSAH, Ama    │ │ │OWUSU, Yaw  │ │ │BOATENG, Adjoa │ │ │ASANTE, Kwame    │ │
│ │PT-000431 M34│ │ │PT-000398 F27  │ │ │PT-000402 M8│ │ │PT-000377 F51  │ │ │PT-000411 M45    │ │
│ │⏱ 12m  ⏳Pend │ │ │⏱ 31m  ✓Synced │ │ │⏱ 18m ✓Sync │ │ │⏱ 6m   ⏳Pend  │ │ │⏱ 9m    ✓Synced  │ │
│ └─────────────┘ │ └───────────────┘ │ └────────────┘ │ └───────────────┘ │ └─────────────────┘ │
│                 │                   │                │                   │                     │
│  [ F2 Start ]   │  [ F3 Consult ]   │  [ F5 Result ] │  [ F7 Collect ]   │  [ F8 Dispense ]    │
└─────────────────┴───────────────────┴────────────────┴───────────────────┴─────────────────────┘
```

- Horizontal scroll container of `flex gap-4` columns, each `w-[300px] shrink-0`. Columns map to workflow stages; the column set is filtered by the signed-in role, so a nurse sees "Waiting vitals" first and a pharmacist sees "Awaiting pharmacy" first.
- Column header is sticky within the board's vertical scroll: stage name, `tnum` count, and the **oldest wait time** in that column. Past a threshold (configurable, default 45 min) the wait time renders `text-danger` **with a `TriangleAlert` icon** — never colour alone.
- Cards sort oldest-first by default, with a one-click "longest waiting first" toggle. Sorted by `queuedAt`.
- **Keyboard:** `F2`/`F3`/`F5`/`F7`/`F8` act on the **focused card** (not the top card — that would be a data-safety hazard). `Tab` moves focus between cards; `ArrowUp`/`ArrowDown` move within a column; `ArrowLeft`/`ArrowRight` move between columns. The focused card gets a `ring-2 ring-ring` ring plus a visible "F3 Consult" hint.
- Board data comes from a single `useLiveQuery` over `encounters` filtered by `workflowStage` and `status !== 'cancelled'`, grouped in memory. One query, because five queries means five chances to be inconsistent.
- **A card never disappears without a trace.** When a transition moves a card to another column, the card animates `opacity` 0→1 in its new column over `--duration-normal` and the **source column count changes with it**. If an action is rejected (a lab result entered for an encounter that has already moved), the card stays put and a `toast` explains the rejection.

### 5.6 Consultation split view

```
┌───────────────────────────────┬──────────────────────────────────────────────┐
│ PATIENT SUMMARY  w-[360px]    │ ACTIVE FORM   flex-1                         │
│ sticky top-14, own scroll     │ scrolls independently                        │
│                               │                                              │
│ [Avatar] AMOAH, Kofi          │  PRESENTING COMPLAINT                        │
│ PT-000431 · VN-2026-007712    │  [____________________________________]      │
│ 34y · Male · O+               │                                              │
│ ⚠ Allergy: Penicillin         │  VITALS (latest, 09:12)                      │
│ ─────────────────────────     │  BP 128/82 · T 38.4 · PR 96 · RR 20 · SpO2 97│
│ TODAY                         │                                              │
│ 09:12 Vitals ✓                │  DIAGNOSIS                                   │
│ 09:20 Triage ✓                │  [ICD-10 combobox________________]           │
│                               │                                              │
│ HISTORY                       │  PLAN                                        │
│ 12 Feb Malaria ✓              │  [ ] Lab order   [ ] Prescription            │
│ 03 Jan ANC ✓                  │  [ ] Admit       [ ] Referral                │
│                               │                                              │
│ [Open full record →]          │  ┌ sticky action bar ─────────────────────┐  │
│                               │  │ Cancel   [ Save & close encounter ]    │  │
│                               │  └────────────────────────────────────────┘  │
└───────────────────────────────┴──────────────────────────────────────────────┘
```

- Left pane `w-[360px] shrink-0 border-r border-border bg-surface`, independently scrollable, `sticky top-14`. It is **read-only** — no inputs, so a clinician cannot mistype into the summary while reading it.
- The allergy banner is always the first thing in the left pane when present: `bg-danger-soft text-danger-fg` with a `TriangleAlert` icon and the word "Allergy".
- Right pane is the form (§5.4 rules) with `max-w-[720px]` inside a wider pane.
- Below `lg` (1024px) the split view collapses: the summary becomes a collapsible `Accordion` above the form, closed by default, and a persistent "Allergies" chip stays visible in the page header.
- The vitals strip in the form is a compact read-only echo of the latest `vitals` row. It is deliberately not editable here: vitals are the nurse's record, and letting a doctor silently overwrite them destroys attribution.

---

## 6. Workflow-specific screen patterns

| Screen | Primary role | Layout | Key interaction |
|---|---|---|---|
| Registration | `front_desk` | Form §5.4, `max-w-[640px]` | Individual ⇄ Family toggle; `Ctrl+Enter` saves |
| Vitals capture | `nurse` | One-row form, `max-w-[880px]` | All 5 vitals, keyboard-only, Enter advances |
| Queue board | all clinical | §5.5 | F-keys act on the focused card |
| Consultation | `doctor` | §5.6 split view | Diagnosis combobox + order checkboxes |
| Lab worklist | `lab_scientist` | Table + result drawer | Result entry in a `Drawer`, never a page change |
| Cashier payment | `cashier` | Two-pane: invoice left, tender right | Change calculated live; print on success |
| Pharmacy dispensing | `pharmacist` | Prescription queue + dispense dialog | Per-line tick-off, partial dispense allowed |
| Admission board | `nurse` / `doctor` | Ward columns of beds | Daily charges accrue; discharge closes |
| Referral composer | `doctor` | Form + live QR preview side by side | Print disabled until QR renders |

### 6.1 Patient registration

- **Toggle first.** A `Radio` `card` pair at the top: **Individual** / **Family**. It is the first control because it changes the rest of the form, and choosing it last means re-entering data.
- **Individual:** surname, first name, other names, sex, date of birth *or* estimated age (a `Checkbox` "Date of birth unknown" swaps the `DatePicker` for a numeric age field plus an "estimated" flag — a real PHC need, and it must be explicit rather than a fake date), phone, address, next of kin.
- **Family:** the same fields for the **head of household**, then a `households` block: family number (auto-generated, read-only, `tnum`), address, and a **family-member list** — a `DataTable` with a trailing `+ Add member` button that appends an inline editable row. Members can be existing patients (via `Combobox` search) or new. The household number appears in the page header badge as soon as it is generated.
- **Duplicate warning:** on blur of surname + phone, query `patients` and, if a likely duplicate exists, show an `Alert` tone `warning` listing the matches with their patient numbers and a "Use existing record" action. Never block registration — a blocked clerk invents a workaround, and two half-records are worse than one duplicate.
- **Save:** `Ctrl+Enter` anywhere; primary button "Register patient". On success, `toast` reads "Registered on this device · will sync when online" — the offline promise, stated positively, at the exact moment the user needs to hear it.

### 6.2 Vitals capture

```
┌───────────────────────────────────────────────────────────────────────────────┐
│  AMOAH, Kofi · PT-000431 · 34y M          VN-2026-007712 · 09:12             │
├───────────────────────────────────────────────────────────────────────────────┤
│  Temp °C     BP (sys/dia)    Pulse bpm    Resp /min     SpO2 %     Weight kg  │
│  [ 38.4 ]    [ 128 ]/[ 82 ]  [  96  ]     [   20  ]     [  97  ]   [  71.5 ]  │
│   36.5-37.5  90-140/60-90    60-100       12-20         95-100                │
├───────────────────────────────────────────────────────────────────────────────┤
│  ⚠ Temperature 38.4 °C is above the expected range                            │
│  Note (optional) [_______________________________________________]            │
│                                          Cancel   [ Save vitals ]  Ctrl+Enter│
└───────────────────────────────────────────────────────────────────────────────┘
```

- **All five vitals in one row** (`grid-cols-5 gap-4`, wrapping to `grid-cols-2` below `md`). One row means one visual sweep, one comparison against the reference range printed under each field.
- **Reference range** under every field, `text-xs text-text-muted`, always visible. This is not a tooltip: a nurse should not have to hover to learn what normal is.
- **Out-of-range is flagged, never blocked.** The field border becomes `border-warning`, an inline `Alert` tone `warning` names the value and the range, and the value is still saved. Blocking an abnormal vital sign would be clinically dangerous.
- **Keyboard-only flow:** the screen autofocuses Temperature with the field's text selected, so the nurse types over it. `Enter` and `Tab` both advance to the next field. The last field's `Enter` submits. `Ctrl+Enter` submits from anywhere. Numeric fields use `inputMode="decimal"` and accept `.` or `,` as the decimal separator.
- **`tnum` and right-aligned** in every numeric field, so digits line up down the row and a transposed `38.4`/`83.4` is easier to spot.
- Saving appends a `vitals` row and advances the encounter from `registered` to `vitals_done` (`00-ARCHITECTURE.md` §7.5). The screen then offers "Next patient" — the fastest possible path back to the queue.

### 6.3 Doctor consultation

- Split view (§5.6). The three-block form is fixed by clinical habit: **Complaint → Examination/Diagnosis → Plan**.
- **Diagnosis** is a `Combobox` over ICD-10 codes, searching on code and description, with the last five codes this doctor used pinned at the top. Free text is allowed in a separate "Clinical impression" `Textarea` but never replaces the coded field.
- **Plan** is a row of `Checkbox`es: Lab order, Prescription, Admit, Referral. Each opens its own `Dialog`; the encounter is not advanced until the dialog is saved or dismissed, and dismissing leaves the checkbox unchecked (no phantom orders).
- **Lab order** `Dialog`: a `DataTable` of available tests with per-row tick boxes, a priority `Radio` (Routine / Urgent), and a "clinical note" `Textarea`. Shows the running count and, where a tariff exists, the running total in `tnum`.
- **Prescription** `Dialog`: drug `Combobox` (name, form, strength), dose, route, frequency, duration, quantity. Each saved line appends to a `prescriptions` row's item list. A duplicate-drug check warns but does not block.
- **Referral** is a separate full-screen composer (§6.9) because it prints.
- Saving sets the encounter to `awaiting_payment` (drugs) or `awaiting_lab`, per the §7.5 state machine. The screen must surface which one it did, with a `toast`: "Sent to cashier · INV-2026-000903".

### 6.4 Lab worklist

- A `DataTable` of ordered tests, columns: Requested (time, `tnum`), Patient (name + number), Test, Priority, Requesting doctor, Status, Sync.
- **Priority and status are `Badge`s with icons**, and "Urgent" sorts to the top of the default view.
- **Result entry happens in a `Drawer` (`right-wide`), not a new page.** The worklist stays visible behind it, so the scientist keeps their place in the queue. Result fields are per-analyte rows with unit and reference range shown, plus a `Textarea` for comments and an optional file attachment.
- **Abnormal results require an explicit acknowledgement** — a `Checkbox` "I have verified this abnormal result" — before the "Release result" primary button enables. This is the one place where the app adds friction on purpose, and you should be able to justify it.
- Releasing advances the encounter to `lab_done` and then back to `awaiting_doctor`, and the worklist row shows "Returned to doctor" with a `toast` confirming it.

### 6.5 Cashier payment collection

```
┌────────────────────────────────┬─────────────────────────────────────────────┐
│ INVOICE  INV-2026-000903       │ PAYMENT                                     │
│ AMOAH, Kofi · PT-000431        │                                             │
│                                │ Total due        ₦  94.00  ← tnum, bold     │
│ Item                Qty Amount │                                             │
│ Consultation          1  50.00 │ Amount received                             │
│ Artemether/Lumef.     1  24.00 │ [ 100.00 ]  ← autofocused, selected         │
│ Paracetamol 500mg    10   5.00 │                                             │
│ Malaria RDT           1  15.00 │ Quick tender                                │
│ ────────────────────────────── │ [ 100.00 ] [ 200.00 ] [ Exact ]             │
│ TOTAL                    94.00 │                                             │
│ PAID                      0.00 │ Change to give                              │
│ BALANCE                  94.00 │ ₦   6.00  ← tnum, text-2xl, success-fg      │
│                                │                                             │
│                                │ Method  ( ) Cash ( ) Transfer ( ) Waiver    │
│                                │                    Cancel [Collect payment] │
└────────────────────────────────┴─────────────────────────────────────────────┘
```

- **Change is calculated live on every keystroke** and displayed at `text-2xl` in `success-fg` with a `tnum` value. This is the single most error-prone calculation at a PHC counter, so it gets the largest type on the screen.
- **Overpayment beyond a sane threshold** (say 10× the balance) raises a `warning` `Alert` inline, not a block.
- **Underpayment** leaves "Collect payment" enabled but relabels it "Collect part payment" and shows the resulting balance in `warning-fg`. Partial payment is legitimate care, so it must be possible in two clicks.
- **Money is never a float.** The tender field is parsed to minor units once, on change, and the change calculation is integer arithmetic. Display goes through `formatMoney`.
- **On success:** append a `payments` row, show `success` `toast` "Payment recorded · receipt RCP-2026-000148", then open the print preview (`PrinterFrame`, §3.3) with the receipt **already rendered**, so the cashier's only remaining decision is whether to print. Which printer is remembered per device, not asked every time.
- **A failed print never loses the payment.** The payment is committed to Dexie before the print dialog opens. If printing fails, the receipt stays in the preview with a "Reprint" button, and the toast says "Payment saved · reprint from the invoice".
- **Void/reverse** is a `DropdownMenu` action on the invoice, requires a `Dialog` with a mandatory reason, and is restricted to `cashier` and `admin` (`accountant` can view). It writes a reversing entry — the original payment is never edited.

### 6.6 Pharmacy dispensing

- Queue of prescriptions in state `awaiting_pharmacy`, as a `DataTable`: Patient, Prescribed (time, `tnum`), Items count, Prescriber, Waiting (computed, `tnum`), Sync.
- Selecting a row opens the **dispense `Dialog`**: one row per prescribed item with drug, dose, frequency, duration, quantity prescribed, and a `quantity dispensed` input defaulting to the prescribed quantity.
- **Partial dispense is a first-class state**, not an error. Lines can be ticked off individually; the dialog footer shows "Dispensing 4 of 5 items" and the primary button becomes "Dispense 4 items". A `Textarea` "Reason for partial" appears when the dispensed total is below the prescribed total.
- **Substitution** (a different brand or formulation) requires a `Combobox` pick plus a free-text reason, and is recorded on the `dispenses` row. There is no inventory logic here (`00-ARCHITECTURE.md` §1.2), so the dialog never claims to know stock levels — this is an explicit non-goal and worth stating in your defence.
- On confirm: append a `dispenses` row, advance the encounter to `pharmacy_done`, and print the **dispensing label** (58mm: patient name, drug, dose, frequency, duration, date, facility) if the pharmacist asks for it. Labels are opt-in; receipts are not.
- Counselling notes entered here are printed on the label only if the pharmacist ticks "Include counselling notes", because thermal paper is short and a 200-character note pushes the drug name off the slip.

### 6.7 Admission daily-charge board

- Ward view: one column per ward, one `Card` per bed. Occupied beds show patient name, patient number, admitted (time), day count (`tnum`, e.g. "Day 3"), and the running charge total.
- **Daily charges accrue on the `admissions` row**, one line per day per tariff item. The board shows today's not-yet-posted charges in `warning-fg` with the word "Pending posting" and a "Post charges" action per bed and a bulk "Post all" for the ward.
- **Discharge** is a `Dialog` that lists every unposted day, requires a final day count confirmation, and shows the resulting balance before it commits. The primary button is "Discharge & post N days".
- Transfers between wards create a new day-line with the new tariff; the UI shows the change as a `Timeline` entry rather than rewriting history.
- The board is `text-4xl` for bed numbers at the ward's wall-mounted screen size, and it is explicitly **desktop/tablet only** (§9).

### 6.8 Consultation ordering shortcuts (the small ones that matter)

- `Ctrl+K` from anywhere → patient search. Selecting a patient from the palette jumps to the most contextually useful screen for the current role: nurse → vitals, doctor → consultation, cashier → billing, `front_desk` → the patient record.
- `F9` on any patient screen → open the full encounter history `Drawer`.
- `Alt+N` → "next patient in my queue". This exists because the single most repeated action at a PHC is "finish this one, start the next one".

### 6.9 Referral note composer

```
┌──────────────────────────────────────┬────────────────────────────────────┐
│ REFERRAL DETAILS   (form, §5.4)      │ PREVIEW    (sticky, print-accurate)│
│                                      │ ┌────────────────────────────────┐ │
│ Refer to *      [Combobox facility ] │ │      REFERRAL NOTE             │ │
│ Department      [Combobox          ] │ │ Riverside Health Centre        │ │
│ Priority *      ( ) Routine ( ) Urgent│ │ REF-2026-000067                │ │
│ Reason *        [Textarea          ] │ │ AMOAH, KOFI  (M, 34y)          │ │
│ Diagnosis *     [ICD-10 Combobox   ] │ │ ┌──────────┐                   │ │
│ Clinical summary[Textarea          ] │ │ │  QR CODE │  ← raster canvas  │ │
│ Attach results  [x] labs  [x] vitals │ │ └──────────┘     28 × 28 mm    │ │
│                                      │ │ Code: 7F3K-9QMD  PIN: 482913   │ │
│ ☑ Include 6-digit PIN                │ └────────────────────────────────┘ │
│                                      │ Paper ( ) 58mm  ( ) 80mm           │
│ Cancel   [ Issue referral & print ]  │ [ Test print ]  [ Print ]          │
└──────────────────────────────────────┴────────────────────────────────────┘
```

- **Validation is strict here** because the referral is consumed by another facility with no ability to ask a follow-up question: refer-to facility, reason and diagnosis are required; the PIN defaults on (`REFERRAL_PIN_REQUIRED`).
- **The QR preview is the real thing.** It is generated by `qrcode` 1.5.4 into a `<canvas>` and the *same* canvas is what the print stylesheet sizes. Never preview an SVG and print a different renderer — that is how you ship a slip whose QR does not scan.
- **`Print` is disabled until the QR canvas has finished rendering** (`onRendered`), with the button label reading "Preparing QR…" in the meantime. Printing a blank QR square is a wasted slip and a lost referral.
- **Expiry is stated on screen and on paper:** "Link expires 48 hours after issue", with the exact local date-time shown next to it.
- The signed-in doctor is the only role that can reach this screen (`00-ARCHITECTURE.md` §6). The composer shows the issuing doctor's name from the session, read-only.
- After printing, a `success` `toast` confirms "Referral REF-2026-000067 issued · expires 16 Feb, 10:05" and the composer offers "Issue another" and "Back to queue".

---

## 7. Print patterns

Printing is a first-class output of this system, not an afterthought: a referral slip that cannot be scanned is a failed referral, and a receipt that is illegible is a billing dispute.

### 7.1 Paper, columns and character budgets

| Paper | Printable width | Mode | Char cell | Columns | We design to |
|---|---|---|---|---|---|
| 58 mm POS | ~48 mm | Font A (12 × 24 dots @ 203 dpi) | 1.50 mm | **32** | **32ch** |
| 58 mm POS | ~48 mm | Font B (9 × 17 dots) | 1.13 mm | 42 | — |
| 80 mm POS | ~72 mm | Font A | 1.50 mm | **48** | **42ch** |
| A4 | 186 mm | any | — | — | 186 mm |

The rule: **compose to the lowest column count you must support.** 58 mm slips are composed to exactly 32 characters; 80 mm slips to 42, which leaves headroom on printers whose Font A gives 48 and still fits the narrower 42-column units some drivers report. Never compose to 48 and hope. The browser's print path is not the printer's native ESC/POS path — you are rendering text into a page box and letting the driver rasterise it — so treat 42 as the safe ceiling.

### 7.2 `web/src/styles/print.css`

```css
/* web/src/styles/print.css
   Imported last in web/src/app/main.css. This is the ONLY file in the project
   permitted to use !important (see section 10). */

/* ---------- Page geometry. Thermal rolls are continuous: height is auto. ---------- */
@page {
  size: 58mm auto;   /* default: the commonest slip in a PHC */
  margin: 0;
}
@page receipt-58 { size: 58mm auto; margin: 0; }
@page receipt-80 { size: 80mm auto; margin: 0; }
@page a4         { size: A4 portrait; margin: 12mm; }

/* Elements opt into a named page with `page: receipt-80;` */
.receipt-58        { page: receipt-58; }
.receipt-80        { page: receipt-80; }
.report-a4         { page: a4; }

/* ---------- Screen/print visibility ---------- */
.print-only { display: none; }

@media print {
  /* 1. Hide every piece of app chrome. Belt and braces: a data attribute for
        ad-hoc cases, plus the structural selectors that would otherwise leak. */
  .no-print,
  [data-print="hide"],
  nav[data-app-sidebar],
  header[data-app-topbar],
  [data-app-pageheader],
  [data-toast-region],
  [data-radix-popper-content-wrapper],
  dialog,
  .printer-toolbar {
    display: none !important;
  }
  /* 2. Unwind the app shell so the slip starts at the top-left of the paper. */
  html, body {
    margin: 0 !important;
    padding: 0 !important;
    width: auto !important;
    background: #FFFFFF !important;
    color: #000000 !important;
  }
  main, .app-main {
    margin: 0 !important;
    padding: 0 !important;
    max-width: none !important;
    overflow: visible !important;
  }
  /* 3. Force black on white. Thermal paper is one bit per dot; a grey is a
        dither pattern, and dithered 8px text is unreadable. */
  * {
    color: #000000 !important;
    background: transparent !important;
    border-color: #000000 !important;
    box-shadow: none !important;
    text-shadow: none !important;
    -webkit-print-color-adjust: exact;
    print-color-adjust: exact;
  }
  /* 4. The receipt itself: fixed-pitch, fixed column count, no wrapping. */
  .receipt {
    font-family: var(--font-mono);
    font-size: 9.5pt;
    line-height: 1.35;
    white-space: pre;          /* the composer already wrapped at the column budget */
    letter-spacing: 0;
    font-variant-numeric: tabular-nums;
    margin: 0 auto;
  }
  .receipt-58 { width: 48mm; }  /* 32ch */
  .receipt-80 { width: 72mm; }  /* 42ch */

  /* 5. Never split a logical row across slips. */
  tr, li, .receipt-line, .receipt-item, .avoid-break {
    break-inside: avoid;
    page-break-inside: avoid;
  }
  h1, h2, h3, .receipt-section {
    break-after: avoid;
    page-break-after: avoid;
  }
  /* 6. Repeat table headers on long (A4) reports. */
  thead { display: table-header-group; }
  tfoot { display: table-footer-group; }
  table { border-collapse: collapse; width: 100%; }
  th, td { border: 1px solid #000; padding: 2mm 1.5mm; font-size: 9pt; }

  /* 7. Links: a printed referral may be the only copy, so show the URL. */
  a[href^="http"]::after { content: " (" attr(href) ")"; font-size: 8pt; }

  /* 8. The QR must never be scaled by a parent, blurred by a transform, or
        clipped. Size in millimetres, integer module scaling only. */
  .qr-canvas {
    display: block !important;
    width: 28mm !important;
    height: 28mm !important;
    margin: 3mm auto !important;
    transform: none !important;
    filter: none !important;
    image-rendering: pixelated;
    opacity: 1 !important;
    break-inside: avoid;
  }
  .qr-caption {
    text-align: center;
    font-size: 8.5pt;
    margin-top: 1mm;
  }
}

/* Print preview on screen: the same component, the same measurements. */
@media screen {
  .print-preview .receipt {
    font-family: var(--font-mono);
    font-size: 11px;
    line-height: 1.35;
    white-space: pre;
    color: #000;
    background: #FFF;
  }
  .print-preview .qr-canvas { width: 28mm; height: 28mm; image-rendering: pixelated; }
}
```

Three things this file deliberately does:

1. **`background: transparent !important` on `*`** kills the sticky-header and zebra-row fills that would otherwise print as grey bands. Specific receipt rules re-add the few black fills that are intentional (a total separator line).
2. **`white-space: pre`** on the receipt body. The composer has already wrapped at 32 or 42 columns; letting the renderer re-wrap produces orphaned amounts on their own line.
3. **Millimetres for the QR and the paper width**, never `px` or `%`. Print CSS pixels are 1/96 in nominally, but drivers and "fit to page" settings routinely rescale, and a QR that prints at 18 mm instead of 28 mm fails to scan.

### 7.3 Receipt layouts

**58 mm payment receipt — 32 columns, monospace.** Every rule line is exactly 32 characters.

```text
================================
        RIVERSIDE PHC
      Riverside Health Centre
   Tel 030 000 0000  Kumasi
================================
Receipt   RCP-2026-000148
Invoice   INV-2026-000903
Date      14 Feb 2026 09:41
Patient   AMOAH, KOFI
PatientNo PT-000431
Visit     VN-2026-007712
Cashier   A. MENSAH
--------------------------------
ITEM                  QTY  AMOUNT
Consultation            1   50.00
Artemether/Lumefant     1   24.00
Paracetamol 500mg      10    5.00
Malaria RDT             1   15.00
--------------------------------
SUBTOTAL                   94.00
DISCOUNT                    0.00
TOTAL DUE                  94.00
CASH TENDERED             100.00
CHANGE                      6.00
--------------------------------
PAID IN FULL
Method: CASH
================================
  Thank you. Get well soon.
  Keep this receipt for your
  next visit.
================================
Sync: PENDING (1 queued)
Printed 14 Feb 2026 09:41
================================
```

**80 mm referral slip — 42 columns, monospace.** Every rule line is exactly 42 characters.

```text
==========================================
              REFERRAL NOTE
         RIVERSIDE PHC - KUMASI
==========================================
Referral  REF-2026-000067
Issued    14 Feb 2026 10:05
Expires   16 Feb 2026 10:05  (48h)
Priority  URGENT
------------------------------------------
PATIENT
  AMOAH, KOFI   (M, 34y)
  PatientNo PT-000431
  Visit     VN-2026-007712
------------------------------------------
REFERRED TO
  Komfo Anokye Teaching Hospital
  Emergency Department
------------------------------------------
REFERRED BY
  Dr. E. OWUSU
------------------------------------------
REASON FOR REFERRAL
  Severe anaemia, Hb 6.2 g/dL.
  Requires transfusion and
  haematology review.
------------------------------------------
DIAGNOSIS
  D50.9  Iron deficiency anaemia
------------------------------------------
        >>> SCAN TO OPEN <<<
      +----------------------+
      |                      |
      |      QR  CODE        |
      |     28 x 28 mm       |
      |                      |
      +----------------------+
    Or open the link and enter:
      Code  7F3K-9QMD
      PIN   482913
------------------------------------------
Do not fold, staple or crease
through the QR code.
Link expires 48 hours after issue.
------------------------------------------
        Printed 14 Feb 2026 10:05
        Sync: SYNCED
==========================================
```

The `+----+` block above is a *mock-up*; in the real component that region is `<canvas class="qr-canvas">`, rendered by `qrcode` — never ASCII art.

### 7.4 The QR code: raster, size, and the darkness warning

**Raster, not vector.** Generate the QR with `qrcode` 1.5.4 into a canvas and hand the print stylesheet a bitmap:

```ts
// web/src/features/referrals/useReferralQr.ts
import QRCode from 'qrcode';

export async function renderReferralQr(canvas: HTMLCanvasElement, url: string) {
  await QRCode.toCanvas(canvas, url, {
    errorCorrectionLevel: 'M',   // ~15% recovery. 'L' is too fragile on thermal paper.
    margin: 2,                   // the quiet zone is REQUIRED by the spec - never 0
    width: 512,                  // ~18 px/mm at 28mm: crisp when the driver downsamples
    color: { dark: '#000000', light: '#FFFFFF' },
  });
}
```

Why raster and 512 px: thermal drivers rasterise the page at the head's dot pitch (203 dpi for the common 58/80 mm units). A vector QR is re-rendered by that path at whatever resolution the driver chooses and its modules can land on fractional dots, producing ragged, merged modules. A 512 px bitmap is downsampled from real pixels, which keeps module edges clean. Why `errorCorrectionLevel: 'M'` and not `'L'`: thermal printing loses dots to head wear, low darkness and paper quality; the recovery budget is what keeps the code scannable. Why `margin: 2`: without a quiet zone, scanner software cannot find the code's edges at all.

**Size.** The QR prints at **25–30 mm** square; 28 mm is the default. At 203 dpi, 28 mm is 224 dots. A version-2 QR (25 modules + 8 quiet-zone modules = 33) gives ~6.8 dots per module — comfortably above the ~4-dot minimum for a reliable read. Do not shrink below 25 mm to save paper: you will spend more time re-printing referrals than you save in paper.

**The warning you must put in your dissertation's evaluation chapter:** *low printer darkness makes QR codes unscannable.* Thermal print density is a firmware setting (usually `GS ( E` or a DIP switch, exposed as "print density" / "darkness" in the driver, typically 1–8). At low darkness the black modules print grey and partially, the quiet zone fills in with speckle, and scanners fail — often intermittently, which is worse than failing outright because staff stop trusting the printer rather than reporting it. Head wear and cheap paper compound it. This is not a software bug and no amount of CSS fixes it, which is precisely why it needs a test procedure and a documented setup step.

**QR print test procedure** — run this once per printer, record the result, and pin the darkness setting in the device setup screen:

1. Print a **QR test slip** from Admin → Devices → Printers → "QR test". It prints the same 28 mm QR at three darkness levels (driver setting low / medium / high) and at three sizes (22 mm, 28 mm, 34 mm), each labelled with its settings.
2. Wait 10 seconds for the print to cool and settle. Fresh thermal print continues to darken for a few seconds; scanning immediately is not representative.
3. Scan each code **5 times** with a phone at ~15 cm under normal room light, holding the slip flat.
4. Record pass/fail per cell in the table below.
5. Choose the **lowest darkness setting that passes 5/5 at 28 mm**, and write it on the printer. A higher setting darkens the print head faster and shortens its life; the lowest passing value is the correct one.
6. Re-run the test at each quarterly maintenance and after any head cleaning or paper-stock change.

| Darkness | 22 mm | 28 mm | 34 mm |
|---|---|---|---|
| 1 (light) | ☐ ☐ ☐ ☐ ☐ | ☐ ☐ ☐ ☐ ☐ | ☐ ☐ ☐ ☐ ☐ |
| 4 (medium) | ☐ ☐ ☐ ☐ ☐ | ☐ ☐ ☐ ☐ ☐ | ☐ ☐ ☐ ☐ ☐ |
| 8 (dark) | ☐ ☐ ☐ ☐ ☐ | ☐ ☐ ☐ ☐ ☐ | ☐ ☐ ☐ ☐ ☐ |

A size/darkness cell **fails** if any of its five scans fails. Do not average.

### 7.5 Print preflight and failure handling

Before any print job the UI checks, in order: a printer is selected for this device; the paper width matches the composed slip (58 mm slip must not go to an 80 mm queue — it prints a narrow column in the middle of wide paper, which still scans but wastes half a roll); the QR canvas has rendered (referrals only); and the referral has not expired. Failures appear in the `PrinterFrame` toolbar as an `Alert` tone `warning` with a one-click fix.

**Printing never blocks a clinical write.** Every record is committed to Dexie before the print dialog opens. A print failure produces a `toast` with a "Reprint" action and leaves the record intact. A jammed printer must never cost a patient their receipt or the facility its payment record.

---

## 8. Accessibility

Accessibility in a data-entry application is not a compliance checkbox; it is throughput. Every accessibility feature below makes a 200-patient day faster for *everyone*.

### 8.1 Keyboard-first operation

The primary user of this app does not want to touch the mouse. Full keyboard operation is the requirement, and the mouse is the fallback.

| Key | Scope | Action |
|---|---|---|
| `Tab` / `Shift+Tab` | global | Move through the tab order (declared order, tabular data included) |
| `Enter` | form | Submit from any single-line input; advance in the vitals row |
| `Ctrl+Enter` | form | Submit from a `Textarea` or anywhere on the form |
| `Escape` | overlay | Close the topmost Dialog / Drawer / Combobox listbox / palette |
| `Ctrl+K` (or `Cmd+K`) | global | Command palette → patient search |
| `Ctrl+S` | form | Save without leaving the form (does not trigger the browser's save dialog) |
| `Alt+N` | patient screens | Next patient in my queue |
| `Alt+1..9` | global | Jump to sidebar item 1–9 |
| `F2` | queue board | Start vitals on the **focused** card |
| `F3` | queue board | Open consultation on the focused card |
| `F5` | lab worklist | Open the result drawer for the focused row |
| `F7` | cashier | Open payment collection for the focused invoice |
| `F8` | pharmacy | Open the dispense dialog for the focused prescription |
| `F9` | patient screens | Toggle the encounter-history drawer |
| `ArrowUp/Down` | queue board | Move focus within a column |
| `ArrowLeft/Right` | queue board | Move focus between columns |

Two rules about these keys:

- **Function keys act on the focused item, never on the top or first item.** `F8` that dispenses whichever prescription is first in the list is a patient-safety bug waiting for a moment of inattention.
- **Every shortcut is discoverable.** Each one is shown in the relevant button's `Tooltip` and listed in a `Ctrl+/` shortcut sheet. A shortcut nobody knows about is not a feature.
- **Bindings are disabled while a modal is open**, except the ones that modal owns.

Global bindings live in one hook, `useGlobalShortcuts()`, mounted once in the app shell, and they never fire when the event target is an input/textarea/contenteditable unless the binding explicitly opts in (`Ctrl+Enter` does; `F3` does not).

```ts
// web/src/app/useGlobalShortcuts.ts - shape, not the full implementation
useHotkeys('ctrl+k, meta+k', openPalette, { preventDefault: true });
useHotkeys('alt+n', nextPatient, { enabled: !anyOverlayOpen });
// Single source of truth so keys cannot drift between screens.
```

### 8.2 Focus

```css
/* Global fallback: if a component forgets, the browser still shows something. */
:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
  border-radius: var(--radius-xs);
}
```

- One ring style everywhere: 2px `ring-primary` (`#0F766E`, 5.47:1 on white), 2px offset. On a primary-filled button the offset uses `ring-offset-surface` (white) so the ring is visible against the teal fill.
- **Never `outline: none`** without an equivalent replacement.
- Focus is never trapped except in a modal, and Radix handles that; `Escape` always restores focus to the trigger.
- On route change, focus moves to the `h1` (with `tabIndex={-1}`) and the page title updates — otherwise a screen-reader user has no idea the view changed.
- The queue board uses a **roving tabindex**: one card is tabbable, arrows move within, so `Tab` does not walk through 200 cards.

### 8.3 `aria-live` regions

Exactly three, and no more — every additional live region makes the others less useful by interrupting more often.

| Region | Politeness | Contents | Rules |
|---|---|---|---|
| Sync status | `polite`, `aria-atomic="true"` | The state label, e.g. "Offline — 12 saved on this device" | Announce **transitions only**, never progress ticks |
| Form error summary | `assertive` | "3 fields need attention" + the list | Written only on a failed submit; focus moves to the summary |
| Save confirmation | `polite` | "Vitals saved on this device" | Fires after a successful Dexie commit |

Field-level errors use `role="alert"` on the error paragraph, wired through `aria-describedby` by `FormField` (§3.1). A field error is announced once, when it appears, and the field gets `aria-invalid="true"`.

```tsx
// web/src/components/ui/live-region.tsx
export const LiveRegion = ({ children, assertive = false }: Props) => (
  <p
    role={assertive ? 'alert' : 'status'}
    aria-live={assertive ? 'assertive' : 'polite'}
    aria-atomic="true"
    className="sr-only"
  >
    {children}
  </p>
);
```

### 8.4 Touch targets and pointer size

- **Minimum 44 × 44 px** interactive area for anything on a touch device. On a fine pointer the control may *look* 32–36 px tall, but its hit area is padded to 44 px via a pseudo-element, so desktop density and touch accuracy coexist:

```css
/* components/ui/touch-target.css */
@media (pointer: coarse) {
  .touch-target { position: relative; }
  .touch-target::after {
    content: '';
    position: absolute;
    inset: 50% auto auto 50%;
    translate: -50% -50%;
    min-width: 44px;
    min-height: 44px;
  }
}
```

- Spacing between adjacent targets is at least 8 px so a near-miss does not hit the neighbour — this matters most for the `DataTable` trailing action menu and the cashier's quick-tender buttons.
- Drag interactions (none are required by this app) would need a keyboard equivalent; if you add one, that is the rule.

### 8.5 Colour is never the only signal

| Meaning | Colour | Also always present |
|---|---|---|
| Synced | `success-soft` | `Check` icon + the word "Synced" |
| Pending | `warning-soft` | `Clock` icon + "Pending" |
| Conflict | `danger-soft` | `TriangleAlert` + "Conflict" |
| Offline | `warning-soft` | `CloudOff` + "Offline — N saved on this device" |
| Out-of-range vital | `warning` border | `Alert` text naming the value and the range |
| Long wait in queue | `text-danger` | `TriangleAlert` + the wait time in text |
| Required field | red asterisk | `required` attribute + `aria-required` |
| Abnormal lab result | `warning-fg` | The word "Abnormal" and the reference range |
| Selected table row | `primary-soft` | `aria-selected` + a left `border-l-2 border-l-primary` |

Test it: screenshot any status-bearing screen and convert it to greyscale. If you cannot tell the states apart, the design fails this section.

### 8.6 Other non-negotiables

- **Labels are visible, not placeholder-only.** A placeholder disappears the moment typing starts; a PHC clerk re-reading a half-filled form needs the label.
- **`lang` is set** on `<html>`; dates and numbers go through `date-fns` and `Intl.NumberFormat`, never manual string building.
- **Images and icons:** decorative lucide icons get `aria-hidden="true"`; an icon that is the only content of a button gets an `aria-label` on the button.
- **Headings are hierarchical:** one `h1`, sections as `h2`, cards as `h3`. No heading level is skipped to get a smaller font — use type tokens for that.
- **Zoom to 200 %** must not break any screen. Because everything is sized in `rem` and layouts wrap rather than fixed-position, this works; verify it on the queue board and the consultation split view specifically.
- **`prefers-reduced-motion`** is honoured globally (§2.6) and verified in at least one Playwright test with `reducedMotion: 'reduce'`.

---

## 9. Responsive strategy

**Desktop/laptop workstations are the primary target.** A PHC's registration desk, cashier's counter and doctor's consulting room all have a fixed machine. Responsive support exists so that a nurse can do a round with a tablet or check something on a phone — not so the app can be "mobile-first". Do not invert the priorities.

| Breakpoint | Width | What changes |
|---|---|---|
| base | < 640px (`sm`) | Sidebar becomes a `Drawer` (bottom sheet) behind a hamburger; top bar collapses the sync label to icon + count; page gutter 16px; forms full width; `DataTable` switches to stacked label/value cards; `Button size="lg"` (44px) everywhere |
| `sm` | ≥ 640px | Sync label shows text again; two-column form pairs allowed; `DataTable` gains horizontal scroll instead of stacking |
| `md` | ≥ 768px | Sidebar becomes a collapsible rail (icons + labels); page header actions move inline instead of stacking; consultation summary becomes a collapsible panel |
| `lg` | ≥ 1024px | **Full desktop layout.** Fixed 240px sidebar; consultation split view (360px summary + form); queue board shows 3 columns without scrolling; forms cap at 640px |
| `xl` | ≥ 1280px | Queue board shows 5 columns; consultation form caps at 720px; dashboards go to a 4-up `StatCard` row |
| `2xl` | ≥ 1536px | Main content caps at 1600px and centres; nothing else changes — extra width buys whitespace, not more columns |

Per-screen honesty about where each screen actually works:

| Screen | 360px phone | Tablet | Desktop | Note |
|---|---|---|---|---|
| Patient lookup / `Ctrl+K` | ✅ usable | ✅ | ✅ | Designed for one-handed use |
| Vitals capture | ✅ usable | ✅ | ✅ | Single row wraps to 2×3 |
| Patient registration | ⚠️ painful | ✅ | ✅ | Too many fields for a phone; works, nobody will enjoy it |
| Consultation | ⚠️ summary only | ✅ | ✅ | Split view collapses below `lg` |
| Lab result entry | ⚠️ readable, not writable | ✅ | ✅ | Result entry needs the width |
| Cashier payment | ⚠️ tight | ✅ | ✅ | The change display survives; the invoice table does not |
| Pharmacy dispensing | ❌ not supported | ✅ | ✅ | Multi-line dialog needs ≥ 768px |
| **Queue board** | ❌ **not supported** | ⚠️ 1–2 columns | ✅ | **Desktop-only by design.** A 5-stage board on a 360px screen is a different product, and pretending otherwise ships a worse one |
| Admission board | ❌ not supported | ✅ | ✅ | Ward board is a wall-mounted or desk screen |
| Referral composer | ❌ not supported | ⚠️ | ✅ | Form + live QR preview side by side needs ≥ 1024px |

Below its supported breakpoint, a desktop-only screen renders a clear `EmptyState` variant: "The queue board needs a wider screen. Open it on a workstation, or use **Queue list**." — with a link to a simpler list view that *is* responsive. Degrade to a working simpler view; never ship a broken complex one.

Implementation rules:

- Breakpoints are Tailwind's `sm/md/lg/xl/2xl` and nothing else. No custom media queries in components.
- Layout uses `flex`/`grid` with `gap`, not margins, so wrapping is automatic. Avoid fixed heights on anything containing text.
- Every table has a plan for small screens: horizontal scroll inside `overflow-x-auto` for ≤ 7 columns, stacked cards for wider tables.
- Test at 360, 768, 1024, 1366 and 1920 px, plus 200 % browser zoom at 1366. Those five widths cover every real PHC machine you will meet.

---

## 10. UI conventions and banned patterns

### 10.1 Banned, with the replacement

| Banned | Why | Use instead |
|---|---|---|
| `window.alert()` | Blocks the main thread, cannot be styled, ignores reduced motion, breaks the app's focus model | `Dialog` for decisions, `toast` for information |
| `window.confirm()` | Same, plus its OK/Cancel wording is unstyleable and easy to misread | `Dialog` with an explicit verb button ("Void payment", not "OK") |
| `window.prompt()` | Cannot validate, cannot show a reason field properly | `Dialog` with a `Textarea` |
| Native `<select>` for > 15 options | No search, no type-ahead, unusable with 400 drugs or 12,000 ICD-10 codes | `Combobox` |
| Native `<select>` at all, where avoidable | Inconsistent rendering across OSes, cannot be styled to match tokens | Radix `Select` |
| Native `<input type="date">` | Rendering and keyboard behaviour differ per browser; no range control | `DatePicker` |
| Spinner with no text | A user cannot tell whether the app is working, stuck, or finished | Spinner **plus** a label ("Loading patients…") |
| Colour-only status | Fails WCAG 1.4.1 and fails in greyscale, poor light, and colour-blindness | Icon + text + colour (§8.5) |
| Dark mode / theme toggle | §1.4 | Light only |
| Animated page/route transitions | Delays the first paint of the screen the user asked for | No route animation. Content fades in at most |
| Animation > 200 ms | §2.6 | `--duration-normal` maximum |
| Animating `height`/`width`/`top` | Layout thrash on low-end hardware | `opacity` + `transform` |
| `!important` outside `print.css` | Destroys the cascade and makes `className` overrides impossible | Fix specificity, or add a variant |
| Inline hex colours (`bg-[#0F766E]`) | Bypasses the token layer, breaks a future retheme, hides contrast failures | Add a token in `tokens.css` |
| Inline `style={{}}` for anything but measured values (e.g. a width from `ResizeObserver`) | Same, plus it cannot be purged or themed | A CVA variant |
| `outline-none` with no replacement ring | Keyboard users lose their place entirely | `focusRing` from §3.0 |
| `div` with `onClick` | Not focusable, not keyboard-operable, invisible to assistive tech | `button`, or a Radix primitive |
| Placeholder used as a label | Disappears on typing | `FormField` label |
| Red for offline | §1.3 — offline is normal | `warning`/neutral tones |
| Modal for the offline grace warning | Steals focus mid-entry | Sticky `Alert` (§4.4) |
| Blocking a clinical action on connectivity | §4.3 — the core promise of the system | Queue it locally and say so |
| `toFixed(2)` on money inline | Float rounding on a billing ledger | `formatMoney(minor)` helper |
| Table cell with a proportional number | Columns stop aligning; outliers hide | `tnum` + right align |
| Disabled button with no explanation | The user cannot tell what to fix | Disabled + hint text, or enable and validate on submit |
| Truncating a patient name with `…` in a queue card | Two patients become indistinguishable | Wrap to two lines, or show patient number prominently |
| Auto-advancing carousels, marquees, background video | Decorative; §1.1 | Nothing |

### 10.2 Conventions to follow

- **Every screen has**: a breadcrumb, one `h1`, one primary action, and a defined empty state.
- **Every list has**: a loading skeleton, an empty state, and a visible total count in `tnum`.
- **Every save has**: an immediate optimistic UI update (via `useLiveQuery`), a `toast` naming what was saved and where it is, and a `SyncBadge` on the resulting row.
- **Every destructive action**: is behind a `DropdownMenu` or secondary position, uses the `danger` variant, requires a typed or explicit confirmation, and is reversible rather than edited (void, never delete).
- **Data comes from Dexie via `useLiveQuery`.** If you are writing `fetch` in a screen component, stop — see `00-ARCHITECTURE.md` §4.3.
- **Category and inventory** are grouped by feature folder: `features/patients/`, `features/queue/`, `features/billing/`, `features/pharmacy/`, `features/lab/`, `features/referrals/`. Shared primitives go in `components/ui/` and are registered in `ui-registry.md`.
- **New tokens go in `tokens.css` first**, then into the component. A component that introduces a colour is a component that has not finished.

---

## 11. Common mistakes to avoid

1. **Treating offline as an error.** Red offline indicators, "connection lost" modals, disabled Save buttons. This contradicts the entire premise of the project (`00-ARCHITECTURE.md` §1). Being offline is the normal operating mode; the UI must be calm about it.
2. **`useEffect` + `fetch` for clinical data.** It bypasses Dexie, breaks `useLiveQuery`'s single source of truth, and silently fails offline. If a screen shows patient data, it reads from Dexie.
3. **A spinner instead of the data you already have.** Writes hit Dexie first and `useLiveQuery` re-renders; there is nothing to wait for. If you show a loading state after a save, you have invented latency.
4. **Animating to look modern.** A 400 ms modal, a sliding page transition, a bouncing toast. At 200 patients a day this is minutes of dead time and it breaks typing rhythm. 200 ms is the ceiling, not the target.
5. **Dark mode "just for the sidebar".** §1.4. Light chrome throughout; a dark sidebar is dark mode with extra steps.
6. **Shipping a receipt you never printed.** A layout that is perfect at 32 characters on screen can still be unreadable if the driver rescales. Print on the actual printer, on actual paper, and scan the actual QR. Put a photo of the printed slip in your dissertation.
7. **Vector or CSS-drawn QR codes.** They look perfect on screen and blur into an unscannable grey square on thermal paper. Raster canvas, 512 px, quiet zone of 2 modules, 28 mm on paper.
8. **Ignoring printer darkness.** The single most common field failure of QR referrals. Document the darkness setting, and use the test procedure in §7.4. "It scanned on my desk" is not a test.
9. **Colour-only status.** Especially `pending` vs `synced` as two coloured dots. Greyscale-test every screen.
10. **`grey-200` as a form-control border.** 1.23:1, fails WCAG 1.4.11. Controls use `--color-border-strong` (3.46:1).
11. **Lightening a button on hover.** White on `primary-600` is 3.74:1 and fails AA. Hover darkens.
12. **Money as a float.** `94.1 * 100` is not 9410. Parse to minor units once, do integer arithmetic, format once.
13. **Letting the queue board silently drop a card.** A transition that fails must leave the card where it was and say why. Silent disappearance destroys trust in the board.
14. **F-keys that act on the top card.** They act on the focused card. `F8` dispensing the wrong patient's prescription is a clinical incident, not a UI bug.
15. **Blocking a registration on a duplicate warning.** Warn, offer the existing record, and let the clerk proceed. A blocked clerk creates a worse duplicate by hand.
16. **Blocking an abnormal vital sign.** Flag it, name the range, save it. Clinical reality beats validation rules.
17. **A modal for the offline grace warning.** It steals focus mid-entry. Sticky `Alert`, non-dismissible once inside five days.
18. **An error message with no next step.** "Sync failed" is useless. Say what failed, what is safe, and what to do: "3 records could not be uploaded. They are safe on this device. Retry."
19. **Building every screen for a phone.** The queue board, dispensing dialog and referral composer are desktop tools. Support them where they work and say so plainly (§9).
20. **Inventing a token in a component.** A stray `#0F766E` or a one-off `shadow-xl` is how a design system dies. Add the token to `tokens.css`, then use it.
