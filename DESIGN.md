---
name: mbox Console
description: Human-first Kubernetes sandbox operations console.
colors:
  canvas: "oklch(0.982 0.006 82)"
  paper: "oklch(0.996 0.004 82)"
  paper-soft: "oklch(0.965 0.009 82)"
  paper-hover: "oklch(0.948 0.011 82)"
  ink: "oklch(0.23 0.011 70)"
  ink-strong: "oklch(0.19 0.012 70)"
  muted: "oklch(0.51 0.013 70)"
  faint: "oklch(0.68 0.012 70)"
  line: "oklch(0.885 0.01 82)"
  line-strong: "oklch(0.81 0.013 82)"
  accent: "oklch(0.46 0.085 154)"
  accent-hover: "oklch(0.41 0.09 154)"
  accent-soft: "oklch(0.91 0.04 154)"
  accent-ink: "oklch(0.27 0.065 154)"
  danger: "oklch(0.55 0.14 26)"
  danger-soft: "oklch(0.96 0.025 26)"
  warning: "oklch(0.64 0.09 78)"
  info-soft: "oklch(0.94 0.018 242)"
  info-ink: "oklch(0.36 0.05 242)"
typography:
  display:
    fontFamily: "ui-sans-serif, -apple-system, BlinkMacSystemFont, Segoe UI, Helvetica, Arial, sans-serif"
    fontSize: "34px"
    fontWeight: 760
    lineHeight: 1.08
    letterSpacing: "-0.022em"
  headline:
    fontFamily: "ui-sans-serif, -apple-system, BlinkMacSystemFont, Segoe UI, Helvetica, Arial, sans-serif"
    fontSize: "20px"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "-0.012em"
  title:
    fontFamily: "ui-sans-serif, -apple-system, BlinkMacSystemFont, Segoe UI, Helvetica, Arial, sans-serif"
    fontSize: "14px"
    fontWeight: 650
    lineHeight: 1.2
    letterSpacing: "0"
  body:
    fontFamily: "ui-sans-serif, -apple-system, BlinkMacSystemFont, Segoe UI, Helvetica, Arial, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.45
    letterSpacing: "0"
  label:
    fontFamily: "ui-sans-serif, -apple-system, BlinkMacSystemFont, Segoe UI, Helvetica, Arial, sans-serif"
    fontSize: "11px"
    fontWeight: 650
    lineHeight: 1.2
    letterSpacing: "0"
  mono:
    fontFamily: "SF Mono, SFMono-Regular, ui-monospace, Menlo, Consolas, monospace"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.35
    letterSpacing: "0"
rounded:
  sm: "4px"
  md: "6px"
  lg: "10px"
  pill: "999px"
spacing:
  xs: "4px"
  sm: "6px"
  md: "8px"
  lg: "10px"
  xl: "12px"
  2xl: "14px"
  3xl: "16px"
  4xl: "18px"
  5xl: "22px"
  6xl: "24px"
  page-x: "30px"
  page-y: "28px"
components:
  button-default:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "0 12px"
    height: "34px"
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.paper}"
    rounded: "{rounded.md}"
    padding: "0 12px"
    height: "34px"
  button-primary-hover:
    backgroundColor: "{colors.accent-hover}"
    textColor: "{colors.paper}"
    rounded: "{rounded.md}"
    padding: "0 12px"
    height: "34px"
  icon-button:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    width: "34px"
    height: "34px"
  panel:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.lg}"
    padding: "0"
  input:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "0 10px"
    height: "38px"
  status-badge:
    backgroundColor: "oklch(0.94 0.008 82)"
    textColor: "{colors.muted}"
    rounded: "{rounded.pill}"
    padding: "2px 9px"
    height: "24px"
  status-badge-running:
    backgroundColor: "{colors.accent-soft}"
    textColor: "{colors.accent-ink}"
    rounded: "{rounded.pill}"
    padding: "2px 9px"
    height: "24px"
---

# Design System: mbox Console

## Overview

**Creative North Star: "The Operations Notebook"**

mbox should feel like a Notion-adjacent operations workspace for real Kubernetes-backed execution, not a decorative SaaS dashboard. The interface is warm, calm, dense, and legible. It uses paper-like surfaces, restrained green state color, precise table structure, and a persistent detail pane so operators can inspect resources without losing context.

The design serves repeated work: creating projects, shaping templates, launching sandboxes, inspecting runtime state, and catching API failures quickly. It should avoid marketing composition, oversized cards, ornamental gradients, and vague empty states. Every screen should make the current resource, state, permission boundary, or next operation visible.

**Key Characteristics:**

- Warm paper workspace with tinted neutrals, not sterile gray or pure white.
- Dense operational layout: rail, tables, summary strip, split detail panel.
- Restrained accent color used only for primary action, selected state, and healthy status.
- Small-radius controls and panels, aligned to practical product UI patterns.
- Copy is direct and task-oriented: no feature explanations inside the app chrome.

## Colors

The palette is a restrained warm-neutral system with one green operational accent and quiet semantic overlays for danger, warning, and selected information.

### Primary

- **Runtime Green** (`oklch(0.46 0.085 154)`): Primary actions, healthy API status, running badges, and the rare moments where the UI needs to say "this is active."
- **Runtime Green Hover** (`oklch(0.41 0.09 154)`): Hover state for primary buttons only. Do not use it as a decorative darker brand color.
- **Soft Runtime Green** (`oklch(0.91 0.04 154)`): Subtle active backgrounds such as running badges and the brand mark fill.
- **Green Ink** (`oklch(0.27 0.065 154)`): Text placed on soft green surfaces.

### Secondary

- **Selection Blue Wash** (`oklch(0.94 0.018 242)`): Selected table rows and resource-type pills. It is informational, not a second brand accent.
- **Selection Blue Ink** (`oklch(0.36 0.05 242)`): Text on selection wash.

### Neutral

- **Canvas Parchment** (`oklch(0.982 0.006 82)`): Page background.
- **Paper** (`oklch(0.996 0.004 82)`): Main panel, table, input, and dialog background.
- **Soft Paper** (`oklch(0.965 0.009 82)`): Sidebar and secondary workspace surfaces.
- **Hover Paper** (`oklch(0.948 0.011 82)`): Navigation, ghost button, and icon hover backgrounds.
- **Ink** (`oklch(0.23 0.011 70)`): Primary text.
- **Strong Ink** (`oklch(0.19 0.012 70)`): Page title text.
- **Muted Ink** (`oklch(0.51 0.013 70)`): Body metadata, labels, secondary values.
- **Faint Ink** (`oklch(0.68 0.012 70)`): Eyebrows and table headers.
- **Hairline** (`oklch(0.885 0.01 82)`): Panel, table, and section dividers.
- **Strong Hairline** (`oklch(0.81 0.013 82)`): Control borders and scroll affordance.

### Tertiary

- **Danger Red** (`oklch(0.55 0.14 26)`): Destructive text, failed badges, and API failure status.
- **Danger Wash** (`oklch(0.96 0.025 26)`): Destructive hover and failed badge background.
- **Warning Amber** (`oklch(0.64 0.09 78)`): Pending or checking status dots.

### Named Rules

**The One Accent Rule.** Runtime Green is the only true accent. Use it for primary action, selected operational success, and healthy status. Do not introduce blue, purple, or orange action colors.

**The Tinted Neutral Rule.** Never use pure black or pure white. Backgrounds, borders, and text should keep the warm paper bias already present in the CSS tokens.

## Typography

**Display Font:** System sans stack: `ui-sans-serif`, `-apple-system`, `BlinkMacSystemFont`, `Segoe UI`, `Helvetica`, `Arial`, `sans-serif`.
**Body Font:** The same system sans stack.
**Label/Mono Font:** `SF Mono`, `SFMono-Regular`, `ui-monospace`, `Menlo`, `Consolas`, `monospace` for IDs, namespaces, runtime refs, and other machine values.

**Character:** Typography should feel native, quiet, and highly scannable. Use weight, compact spacing, and alignment to create hierarchy instead of display fonts or decorative type.

### Hierarchy

- **Display** (760, `34px`, `1.08`, `-0.022em`): Page-level title only.
- **Headline** (700, `20px`, `1.2`, `-0.012em`): Main panel headings.
- **Detail Headline** (700, `18px`, `1.2`, `-0.012em`): Detail pane selected resource title.
- **Title** (650, `14px`, `1.2`): Row titles, brand name, and strong empty-state text.
- **Body** (400, `14px`, `1.45`): Notes, helper copy, empty-state descriptions. Keep prose around 65 to 75 characters per line where possible.
- **Table Body** (400 to 650, `13px`): Operational records. Use tabular numeric behavior for counts and IDs.
- **Label** (650, `11px` to `12px`): Eyebrows, table headers, form labels, chips, and metadata.
- **Mono** (400, `12px`, `1.35`): Slugs, namespaces, runtime references, and IDs.

### Named Rules

**The Native Tool Rule.** Do not add display fonts, uppercase label systems, or decorative letter spacing. mbox should feel like a serious tool someone can use all day.

**The Data Stays Quiet Rule.** Machine values use mono at small sizes. They should be findable, not loud.

## Elevation

mbox is flat by default and uses structure before shadow: borders, table dividers, tonal paper layers, and a persistent split pane create depth. Shadows are reserved for floating surfaces that truly leave the page plane, currently dialogs and toasts.

### Shadow Vocabulary

- **Popover Shadow** (`0 18px 48px oklch(0.23 0.011 70 / 0.12), 0 2px 8px oklch(0.23 0.011 70 / 0.06)`): Dialogs, toasts, and future command palettes.

### Named Rules

**The Flat-By-Default Rule.** Panels, tables, summary strips, and detail panes do not cast shadows. They sit on paper layers and are separated by hairline borders.

**The Floating Surface Rule.** Use shadow only when the component blocks, overlays, or confirms an action above the workspace.

## Components

### Buttons

- **Shape:** Small product radius (`6px`) with at least `34px` height.
- **Primary:** Runtime Green background and border, Paper text, `0 12px` padding. Use for refresh, launch, create, and final submit actions.
- **Default / Ghost:** Paper background, Strong Hairline border, Ink text. Use for secondary operations, inspect actions, cancel buttons, and non-destructive toolbar controls.
- **Danger:** Paper background with Danger Red text at rest, Danger Wash on hover. Use only for destructive actions such as deleting a sandbox.
- **Hover / Focus:** Hover uses Hover Paper or Runtime Green Hover. Focus uses a 2px green outline with 2px offset.
- **Active:** Controls scale down slightly (`0.96` for buttons, `0.98` for nav links) to confirm input without becoming playful.

### Chips

- **Status Badge:** Pill shape, small dot, compact label, `24px` minimum height.
- **Running:** Soft Runtime Green background with Green Ink text.
- **Failed:** Danger Wash background with Danger Red text.
- **Neutral:** Very light warm neutral background with Muted Ink text.
- **Resource Type:** Selection Blue Wash with Selection Blue Ink, used in the detail panel to identify project, template, or sandbox.

### Cards / Containers

- **Panels:** Use Paper background, Hairline border, `10px` radius, and no shadow.
- **Summary Strip:** One contiguous bordered surface divided by internal hairlines. Do not turn the four metrics into independent cards.
- **Sidebar:** Soft Paper background with a right border on desktop and bottom border on mobile.
- **Detail Pane:** Slightly deeper paper layer (`oklch(0.972 0.006 82)`) with a structural border. It is part of the workspace, not a modal.
- **Internal Padding:** Panels use `16px 18px` headers and `12px 18px` table cells. Page padding is `28px 30px 44px` on desktop and `22px 16px 34px` on mobile.

### Inputs / Fields

- **Style:** Paper background, Strong Hairline border, `6px` radius, `38px` minimum height, `0 10px` horizontal padding.
- **Labels:** `12px`, 600 weight, Muted Ink. Labels sit above controls with a `6px` gap.
- **Focus:** Match buttons with a 2px green outline and 2px offset.
- **Checkboxes:** Native checkbox with Runtime Green accent.
- **Dialogs:** Use native dialog behavior, Paper surface, `10px` radius, Popover Shadow, and a dim warm ink backdrop.

### Navigation

- **Rail:** Fixed left rail on desktop, stacked top rail on mobile. Keep navigation compact and predictable.
- **Links:** `32px` minimum height, `6px` radius, warm muted text. Hover uses Hover Paper and Ink.
- **Brand Mark:** Small `30px` square, Soft Runtime Green background, Green Ink text, serif lowercase `m`. It is a quiet signpost, not a logo hero.
- **API State:** Status dot plus short text at the bottom of the rail. Dot colors follow Warning, Runtime Green, and Danger.

### Tables

- **Structure:** Full-width tables with fixed layout on desktop and internal horizontal scroll on mobile.
- **Headers:** `11px`, 650 weight, Faint Ink, warm header wash, no uppercase transformation.
- **Cells:** `13px`, `12px 18px` padding, top hairline dividers.
- **Selection:** Selected rows use Selection Blue Wash. Hover must not override selected state.
- **Actions:** Keep row actions right-aligned. Destructive actions remain visually quiet until hover.

### Loading, Empty, and Error States

- **Skeletons:** Use warm neutral shimmer bars inside the existing table structure. Do not center a spinner in the page.
- **Empty States:** Use a short strong line plus one practical next-step sentence.
- **API Errors:** Preserve the table location, explain the failing resource, and point users back to the API server or refresh flow.
- **Toast:** Bottom-right floating surface with dark warm ink background, Paper text, and Popover Shadow.

## Do's and Don'ts

### Do

- Use tables, split panes, summary strips, and structured forms for operational work.
- Keep the UI dense but readable. mbox is a console, not a landing page.
- Prefer warm paper surfaces, hairline borders, and stable alignment over shadow-heavy cards.
- Use Runtime Green sparingly for primary action and healthy state.
- Keep resource IDs, namespaces, runtime refs, and slugs in mono type.
- Keep loading and empty states inside the component that owns the data.
- Preserve mobile behavior: no page-level horizontal overflow, tables scroll internally.
- Respect reduced motion with near-zero animation duration.

### Don't

- Do not create a marketing hero, feature-card grid, or decorative dashboard cover for the main app.
- Do not add nested cards, side-stripe accent borders, gradient text, glass effects, or ornamental blobs.
- Do not introduce additional action colors without revisiting the palette.
- Do not use pure `#000` or `#fff`.
- Do not replace familiar controls with custom novelty controls.
- Do not use modals as the first answer for resource inspection. Prefer the detail pane or inline editing when feasible.
- Do not make inactive states saturated or visually louder than the record data.
- Do not add page-load choreography. Motion should only convey state and feedback.
