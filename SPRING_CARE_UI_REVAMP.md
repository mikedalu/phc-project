# Spring Care Hospital — UI/UX Revamp & Design System Specification

> **Target Application:** Spring Care Hospital Clinical Management & Patient Records System  
> **Status:** Canonical Architectural Design Guide & Token Specification  
> **Location:** Root directory guideline for frontend revamps  
> **Scope:** Brand Identity, Landing Page Spec, Design Tokens (Tailwind CSS v4), Ergonomic Clinical Flows, Component Guidelines, and AI Prompt Blueprint.

---

## 1. Executive Summary & Brand Identity

**Spring Care Hospital** is a multi-specialty healthcare facility operating in both connected and resource-constrained environments. The design revamp elevates the application from a bare administrative interface into a **cutting-edge, high-trust clinical operating system**.

### Brand Personality
- **Calm Authority:** Deep medical teals and emerald accents paired with clean, glare-resistant surfaces.
- **Cognitive Clarity:** High-contrast typography, strict visual hierarchies, and instantaneous visual feedback.
- **Resilient Fluidity:** Modern glass-tinted headers, smooth subtle state transitions (<150ms), and unambiguous offline-first indicators.

---

## 2. Design Philosophy: The "Clinical Ergonomics" Model

```
   ┌────────────────────────────────────────────────────────┐
   │             SPEED & RELIABILITY (Non-negotiable)       │
   │   - Zero animation lag on entry (<100ms)               │
   │   - Tabular figures for all vitals and finance         │
   │   - Unambiguous offline vs. cloud sync indicators      │
   └───────────────────────────┬────────────────────────────┘
                               │
               ┌───────────────┴───────────────┐
               ▼                               ▼
   ┌───────────────────────┐       ┌───────────────────────┐
   │   STAFF PRODUCTIVITY  │       │   MODERN AESTHETICS   │
   │ - 1-Click Role Switch │       │ - Refined typography  │
   │ - Global Command (⌘K) │       │ - Subtle elevation    │
   │ - Dense clinical data │       │ - Soft frosted pills  │
   └───────────────────────┘       └───────────────────────┘
```

1. **Information Density over Blank White Space:** Doctors and triage nurses need patient vitals, active medications, and queue statuses without scrolling endlessly.
2. **Keyboard-First Workflows:** Registration and triage forms must be operable entirely with `Tab`, `Space`, `Enter`, and shortcut keys.
3. **Glanceable Status Signalling:** Normal vitals remain neutral; abnormal readings elevate with precise, accessible alert borders and badges.
4. **Offline Dignity:** Working without internet is an engineered mode of operation, not an error. The UI stays calm, professional, and confidence-inspiring during network drops.

---

## 3. Design Tokens (Tailwind CSS v4 Specification)

Add or update these tokens directly in `web/src/styles/tokens.css` inside the `@theme` block.

```css
@theme {
  /* ── 1. Neutral Base (Slate Ramp - Crisp & Glare Resistant) ── */
  --color-slate-50:  #F8FAFC;   /* Main canvas background */
  --color-slate-100: #F1F5F9;   /* Cards, panels, sunken surfaces */
  --color-slate-200: #E2E8F0;   /* Borders, dividers, subtle card borders */
  --color-slate-300: #CBD5E1;   /* Hover borders, disabled controls */
  --color-slate-400: #94A3B8;   /* Placeholders, secondary icons */
  --color-slate-500: #64748B;   /* Secondary captions and labels */
  --color-slate-600: #475569;   /* Standard body copy */
  --color-slate-700: #334155;   /* Strong body, table headers */
  --color-slate-800: #1E293B;   /* Sub-headings, active tabs */
  --color-slate-900: #0F172A;   /* Primary headings, dark badges */

  /* ── 2. Spring Care Primary Ramp (Modern Medical Teal/Emerald) ── */
  --color-spring-50:  #ECFDF5;  /* Soft badge fills, active row highlights */
  --color-spring-100: #D1FAE5;  /* Tinted button backgrounds */
  --color-spring-200: #A7F3D0;  /* Subtle selection boundaries */
  --color-spring-400: #34D399;  /* Online connectivity indicator */
  --color-spring-600: #059669;  /* Accent icons, active navigation */
  --color-spring-700: #047857;  /* Primary Interactive Buttons (Contrast > 5.5:1) */
  --color-spring-800: #065F46;  /* Primary button hover */
  --color-spring-900: #064E3B;  /* Deep header accents, active states */

  /* ── 3. Clinical Semantic Tokens ── */
  --color-danger-50:   #FEF2F2;
  --color-danger-100:  #FEE2E2;
  --color-danger-500:  #EF4444;
  --color-danger-700:  #B91C1C; /* Critical vitals, emergency triage */

  --color-warning-50:  #FFFBEB;
  --color-warning-100: #FEF3C7;
  --color-warning-600: #D97706; /* Pending lab results, expiring referrals */

  --color-info-50:     #EFF6FF;
  --color-info-100:    #DBEAFE;
  --color-info-700:    #1D4ED8; /* Routine observations */

  /* ── 4. Elevation & Glassmorphism ── */
  --shadow-subtle: 0 1px 2px 0 rgb(15 23 42 / 0.05);
  --shadow-card:   0 1px 3px 0 rgb(15 23 42 / 0.08), 0 1px 2px -1px rgb(15 23 42 / 0.08);
  --shadow-float:  0 10px 15px -3px rgb(15 23 42 / 0.08), 0 4px 6px -4px rgb(15 23 42 / 0.04);
  --shadow-modal:  0 20px 25px -5px rgb(15 23 42 / 0.1), 0 8px 10px -6px rgb(15 23 42 / 0.1);

  /* ── 5. Typography & Metrics ── */
  --font-sans: 'Plus Jakarta Sans', 'Inter', system-ui, -apple-system, sans-serif;
  --font-mono: 'JetBrains Mono', 'SF Mono', Consolas, monospace;
}
```

---

## 4. The Spring Care Hospital Landing Page Architecture

The landing page (`/`) serves a dual purpose: a welcoming **reception & triage portal** for the hospital and an **ergonomic cockpit** when clinical staff are on duty.

### 4.1 Page Layout Grid

```
┌────────────────────────────────────────────────────────────────────────────┐
│ [Logo] Spring Care Hospital         [Emergency Call] [Sync Pill] [Sign In] │ ◄ TOP NAV
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│   WELCOME TO SPRING CARE HOSPITAL                                          │
│   Excellence in Clinical Care • Fast-Track Patient Registration            │
│                                                                            │
│   [ 🔍 Search Patient ID, Phone or Name... (⌘K)                          ] │
├────────────────────────────────────────────────────────────────────────────┤
│  FACILITY PULSE (Live Dexie Metrics)                                       │
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌────────────────┐  │
│  │ Active Triage │ │ Doctor Queue  │ │ Pending Labs  │ │ Beds Occupied  │  │
│  │      14       │ │       8       │ │       5       │ │    18 / 24     │  │
│  └───────────────┘ └───────────────┘ └───────────────┘ └────────────────┘  │
├────────────────────────────────────────────────────────────────────────────┤
│  DEPARTMENT WORKSTATIONS (Fast Access)                                     │
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌────────────────┐  │
│  │ 🏥 Reception  │ │ 🩺 Outpatient │ │ 🧪 Laboratory │ │ 💊 Pharmacy    │  │
│  │ Quick Intake  │ │ Consultations │ │ Orders/Results│ │ Fast Dispense  │  │
│  └───────────────┘ └───────────────┘ └───────────────┘ └────────────────┘  │
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌────────────────┐  │
│  │ 💳 Cashier    │ │ 🛏️ Inpatient  │ │ 📋 Referrals  │ │ ⚙️ Admin Logs  │  │
│  │ Billing/POS   │ │ Ward Round    │ │ Digital QR    │ │ Audit Trail    │  │
│  └───────────────┘ └───────────────┘ └───────────────┘ └────────────────┘  │
└────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Key Landing Page Components

1. **Clinical Navigation Bar:**
   - **Hospital Brand Unit:** Hospital crest + "Spring Care Hospital" with clean "Primary & Emergency Care" subtext.
   - **Facility Operational Mode Pill:**
     - Connected: `🟢 Cloud Synced (12s ago)`
     - Local/Offline: `🟡 Local Storage Mode (Ready to Sync)`
   - **Staff Session Avatar & Role Badge:** Instant visibility of who is logged into this physical workstation.

2. **Shift Command Banner (Hero):**
   - Clean gradient border (`from-spring-100 to-slate-100`).
   - Global Quick Search field supporting Patient UUID, Hospital Number (`SCH/2026/...`), or Surname.
   - 1-Click Action Buttons: `+ New Patient Registration` (`Alt + N`) and `Start Triage` (`Alt + T`).

3. **Facility Pulse Grid (4 Responsive Metric Cards):**
   - Live Dexie queries responding instantly to local data writes.
   - Clickable metrics that navigate directly into filtered lists (e.g., clicking "Pending Labs" opens the Lab Queue directly).

4. **Department Navigation Tiles:**
   - Visual icons, current queue count chip, and quick descriptions.
   - Role-aware: unpermitted modules display as subdued with a clear tooltip.

---

## 5. Staff-First Navigation & UX Optimization

### 5.1 Role-Optimized Entry Points

| Role | Default Landing Destination | Top 2 Keyboard Shortcuts | Primary Objective |
|---|---|---|---|
| **Front Desk** | `/patients/new` | `Alt+N` (New), `Enter` (Submit) | Rapid check-in under 45 seconds |
| **Nurse** | `/queue?stage=registered` | `Alt+V` (Record Vitals) | Immediate triage without mouse hunting |
| **Doctor** | `/consultations/$encounterId` | `Alt+S` (Save SOAP), `Alt+R` (Refer) | High-density clinical timeline |
| **Lab Scientist** | `/lab` | `Alt+C` (Collect), `Alt+E` (Enter Result) | Barcode/ID-driven specimen queue |
| **Pharmacist** | `/pharmacy` | `Alt+D` (Dispense) | Verification against prescription |
| **Cashier** | `/billing` | `Alt+P` (Post Payment) | Instant invoice allocation |

### 5.2 Global Command Palette (`Cmd + K` / `Ctrl + K`)
A floating modal allowing any hospital worker to:
- Find any patient in under 2 keystrokes (`"John D..."`).
- Jump to any department (`"> Pharmacy"`).
- Execute emergency shortcuts (`"! Emergency Registration"`).

---

## 6. Implementation Blueprint & AI Prompt

This prompt can be copied and provided to any AI engineer or agent to execute the revamp:

```markdown
### AI Execution Prompt: Spring Care Hospital UI Revamp

**Context:**
We are upgrading the web frontend for Spring Care Hospital (`web/src/`). The application is an offline-capable clinical records system using React 19, Vite, Tailwind CSS v4, TanStack Router, Dexie.js, and Lucide React.

**Task Requirements:**

1. **Update Design System Tokens (`web/src/styles/tokens.css`):**
   - Implement the `spring-*` teal/emerald palette as the primary theme.
   - Ensure high contrast (WCAG AA/AAA) across all button text, cards, and clinical badges.
   - Ensure `tabular-nums` is applied to all numbers, currency figures, and vital sign displays.

2. **Revamp the Main Landing & Dashboard Page (`web/src/routes/index.tsx`):**
   - Rebrand the facility title to **Spring Care Hospital**.
   - Create a modern, welcoming Hero section with high-efficiency quick-action cards (`Register Patient`, `Open Queue`, `Record Vitals`, `Lab Tests`, `Pharmacy`, `Cashier`).
   - Integrate live Dexie counters into high-aesthetic "Facility Pulse" indicator cards.
   - Add a subtle status pill showing network state and sync health cleanly.

3. **Modernize Header & Chrome Layout (`web/src/routes/__root.tsx`):**
   - Add Spring Care Hospital branding with a medical cross/leaf glyph.
   - Include quick workstation indicator and current logged-in clinician role pill.
   - Keep interactions lightning fast (<100ms) with clean Tailwind v4 transitions.

4. **Preserve All Clinical Rules:**
   - Keep Light Mode as the primary crisp clinical theme.
   - Do NOT break any Dexie live queries or offline sync operations.
   - Keep all table and vital inputs keyboard-navigable (`autofocus`, `tabIndex`).
```

---

## 7. Verification Checklist

- [ ] All primary buttons meet WCAG 2.1 AA contrast requirements against white backgrounds (>4.5:1).
- [ ] Tabular numbers (`tabular-nums`) are active on all numeric displays.
- [ ] Dexie live queries update the dashboard in real-time without layout shifts.
- [ ] Offline status changes gracefully without disruptive red error screens.
- [ ] The landing page renders instantaneously on low-power workstation terminals.
