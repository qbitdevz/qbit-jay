# qbit-jay design system

Synced 2026-11-27 for `odd/tasks/qbit-jay-identity.md` (T1). Read-only sources:

- `D:\Proyectos\Propios\qbitdev-brand\tokens.css` (CSS custom properties)
- `D:\Proyectos\Propios\qbitdev-brand\tokens.json` (same values plus wordmark and motion numbers)
- `D:\Proyectos\Propios\qbitdev-brand\brand.md`, `mascot\README.md`, `manual\manual-de-marca.html` (rules)

qbitdev values are copied unchanged. Everything under "qbit-jay additions" is new to this project and is never a qbitdev value.
The existing Engram dashboard CSS (`internal/cloud/dashboard/static/*.css`) is upstream's and is not a source: it will be replaced by `internal/qbit/theme` in T0.4, which takes its tokens from this file.

## Colors, dark (qbitdev)

| Token | Value | Usage | Source |
| --- | --- | --- | --- |
| `--qb-bg-deep` | `#04070D` | Page background of everything digital; the only ground for glow | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-panel` | `#0B121B` | Cards, panels, isotype disc and tile | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-line-dark` | `#16222F` | Borders, dim circuit and grid lines | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-blue` | `#2D7FFF` | Gradient start | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-cyan` | `#35E0FF` | Gradient middle, glow, rim light, active state | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-green` | `#3DFF8B` | Gradient end, mono labels | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-teal` | `#1FB6C9` | Secondary accent | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-ink` | `#E8F2FF` | Text on dark | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-ink-muted` | `#8DA2B8` | Secondary text on dark | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-white` | `#FFFFFF` | Pure white (sticker border, light panels) | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |

## Colors, light (qbitdev)

| Token | Value | Usage | Source |
| --- | --- | --- | --- |
| `--qb-light-bg` | `#F4F8FC` | Page background in light mode | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-light-panel` | `#FFFFFF` | Cards and panels in light mode | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-light-line` | `#D6E0EA` | Borders in light mode | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-light-blue` | `#1F6FEB` | Emphasis and links in light mode | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-light-cyan` | `#0FA6C7` | Lines only (2.7:1 on light, too low for text) | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-light-green` | `#17B36A` | Lines only (2.7:1 on white, too low for text) | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-light-ink` | `#0B1521` | Text and ink outline in light mode | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-light-ink-muted` | `#4F6275` | Secondary text in light mode | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |

## Gradient and glow (qbitdev)

| Token | Value | Usage | Source |
| --- | --- | --- | --- |
| `--qb-gradient` | `linear-gradient(135deg, var(--qb-blue) 0%, var(--qb-cyan) 50%, var(--qb-green) 100%)` | Primary button, line under a wordmark, emphasis; always blue, cyan, green in that order | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-glow-soft` | `0 0 24px rgba(53, 224, 255, 0.33)` | Soft glow on small elements | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-glow-strong` | `0 0 18px #35E0FF, 0 0 48px rgba(53, 224, 255, 0.4)` | The one strong glow of a piece | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |

## Typography (qbitdev brand fonts)

| Token | Value | Usage | Source |
| --- | --- | --- | --- |
| `--qb-font-display` | `"Outfit", system-ui, sans-serif` | Wordmark and headlines, weight 500 to 700, lowercase wordmark | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-font-body` | `"Inter", system-ui, sans-serif` | Body text and buttons, weight 400 to 600 | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-font-mono` | `"JetBrains Mono", ui-monospace, monospace` | Labels, code, handles | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |

Numbers from `D:\Proyectos\Propios\qbitdev-brand\tokens.json`: wordmark weight 500, tracking -0.01em (qbitdev's own wordmark). `brand.md` sets display tracking at -0.02em and weight range 500 to 700. The qbit-jay wordmark uses 600 and -0.02em, inside the `brand.md` range (see open points).

Artboards use system font stacks only (gentle-design hard rule). They express the personality through weight, tracking and size, and each artboard's tradeoff note names the real fonts for implementation. The stacks are listed under "qbit-jay additions".

## Radius (qbitdev)

| Token | Value | Usage | Source |
| --- | --- | --- | --- |
| `--qb-radius-field` | `12px` | Inputs, small chips | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-radius-card` | `20px` | Cards and panels | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-radius-pill` | `999px` | Pills, badges | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |

## Motion (qbitdev)

| Token | Value | Usage | Source |
| --- | --- | --- | --- |
| `--qb-ease-standard` | `cubic-bezier(0.2, 0.8, 0.2, 1)` | Default transitions | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-ease-enter` | `cubic-bezier(0, 0, 0.2, 1)` | Entrances (14 px, 470 ms in `tokens.json`) | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-ease-exit` | `cubic-bezier(0.4, 0, 1, 1)` | Exits | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |
| `--qb-ease-sine` | `cubic-bezier(0.45, 0, 0.55, 1)` | Breathing glow loop (4000 ms in `tokens.json`) | `D:\Proyectos\Propios\qbitdev-brand\tokens.css` |

## Spacing

The sources define no spacing tokens, so none are recorded. The scale for `internal/qbit/theme` is still to be decided in T0.4; artboards use plain pixel values and no spacing token.

## Shadows

The only shadows in the sources are the two glows above. Light mode has no glow (`brand.md`). The sticker direction uses a flat 8 px offset shadow derived from `--qb-light-ink` at 12% opacity; it is a qbit-jay choice, not a qbitdev value.

## qbit-jay additions

Not in the qbitdev sources. They come from the reference photo (`odd/tasks/qbit-jay-identity.md` L4, A5) and never replace a qbitdev token. Marked "addition" so they are never mistaken for brand values.

| Token | Value | Usage | Source |
| --- | --- | --- | --- |
| `--jay-blue` | `#2F6FDB` | Addition. Crest, head cap, back and wing coverts | qbit-jay addition, from the reference photo |
| `--jay-blue-deep` | `#1B459E` | Addition. Flight feathers, tail, shadow facets; decorative fill, never the only boundary (2.3:1 on `--qb-bg-deep`) | qbit-jay addition, from the reference photo |
| `--jay-blue-sky` | `#5B9BFF` | Addition. Highlight facets in the hexagon direction | qbit-jay addition, from the reference photo |
| `--jay-mask` | `#070B12` | Addition. Eye line, necklace, wing and tail barring | qbit-jay addition, from the reference photo |
| `--jay-beak` | `#1C2633` | Addition. Beak, legs fill | qbit-jay addition, from the reference photo |
| `--jay-beak-hi` | `#3A4A5E` | Addition. Beak top highlight, legs on dark | qbit-jay addition, from the reference photo |
| `--jay-face` | `#F2F7FD` | Addition. Face, wing bars, tail tip | qbit-jay addition, from the reference photo |
| `--jay-belly` | `#B8C5D6` | Addition. Belly and chest; shade uses `--qb-ink-muted` | qbit-jay addition, from the reference photo |
| `--jay-acorn` | `#D9923A` | Addition. Acorn nut, the memory motif (A1, S4) | qbit-jay addition |
| `--jay-acorn-cap` | `#8A5427` | Addition. Acorn cap and stem | qbit-jay addition |
| `--jay-bark` | `#5B3A1E` | Addition. Branch, acorn hatching | qbit-jay addition |
| `--jay-font-display` | `system-ui, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif` | Addition. Artboard stand-in for Outfit | qbit-jay addition (gentle-design system-font rule) |
| `--jay-font-body` | `system-ui, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif` | Addition. Artboard stand-in for Inter | qbit-jay addition (gentle-design system-font rule) |
| `--jay-font-mono` | `ui-monospace, "Cascadia Code", Consolas, monospace` | Addition. Artboard stand-in for JetBrains Mono | qbit-jay addition (gentle-design system-font rule) |

## Contrast (WCAG AA)

Computed from the values above. Text needs 4.5:1, graphics and large text 3:1.

| Pair | Ratio | Verdict |
| --- | --- | --- |
| `--qb-ink` on `--qb-bg-deep` | 17.8:1 | Pass |
| `--qb-ink-muted` on `--qb-bg-deep` / `--qb-panel` | 7.7:1 / 7.2:1 | Pass |
| `--qb-cyan` on `--qb-bg-deep` | 12.7:1 | Pass |
| `--qb-light-ink` on `--qb-light-bg` | 17.2:1 | Pass |
| `--qb-light-ink-muted` on `--qb-light-bg` | 5.9:1 | Pass |
| `--qb-light-blue` on `--qb-light-panel` | 4.6:1 | Pass for text |
| `--qb-light-blue` on `--qb-light-bg` | 4.3:1 | Fails for body text; use on white, or for large text and lines only |
| `--qb-light-cyan`, `--qb-light-green` on light | 2.7:1 | Lines only |
| `--jay-blue` on `--qb-bg-deep` / `--qb-panel` | 4.2:1 / 4.0:1 | Graphics only (3:1), no text |
| `--qb-light-ink` on `--qb-cyan` (sticker disc) | 11.6:1 | Pass |

## Component rules

From qbitdev (`brand.md`, `manual-de-marca.html`, `mascot/README.md`):

- One strong glow per piece, only on `--qb-bg-deep` or `--qb-panel`. Light mode has no glow.
- The brand gradient is 135 degrees blue, cyan, green, in that order; light mode uses `--qb-light-blue` as flat emphasis.
- Wordmark is lowercase and one word; never split or capitalise it. Prose may write the name capitalised.
- Clear space is half the height of the isotype; nothing enters it. Minimum sizes for qbitdev: horizontal logo 120 px wide, isotype with orbit 32 px.
- One character per piece. bit0 and the qbitdev Q never appear in jay pieces; the line "by qbitdev" is the only endorsement, lowercase, under the wordmark.
- Below 96 px tall a mascot uses its head only (bit0 rule, adopted for the jay).
- Motion: 4 s breathing loop for glow only, and always stopped by `prefers-reduced-motion`.

qbit-jay (decided here, to confirm in T3):

- Isotype is the jay head. Minimum 32 px; clear space adopted by analogy with qbitdev's rule: half the isotype height.
- The glowing rim belongs to the dark theme. On light surfaces the jay appears either with a thick ink outline and white border (sticker) or inside a dark disc/tile.
- The acorn is the recurring motif for memory. Before the 3,000 to 5,000 acorns figure appears in public copy, cite a source (`docs/BRAND.md`). Banner copy in the artboards avoids the number.
- Fork rule (S5): no Engram logos, no Engram elephant, no magenta rim; style genre only.
