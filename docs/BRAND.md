# Brand plug-in points

The visual identity of qbit-jay was chosen on 2026-11-27 (direction 03, see
"Chosen identity"). The final assets live in `assets/branding/`. The README,
the TUI, the web dashboard and the docs use them, and Engram's logos and
screenshots have been removed (task T0.4). This document records where the
identity plugs in.

Status: applied in T0.4; the README header uses `qbit-jay-banner.gif`.

## Name and story

`jay` is the jay bird (Spanish *arrendajo*), chosen for its memory. Two facts carry
the story:

- Episodic memory: scrub jays were the first animals shown to remember **what**
  they hid, **where** and **when** (Clayton and Dickinson, *Nature*, 1998). This
  comes from the specification, section 1.
- Spatial memory, the user's favourite (2026-11-27), quoted as given: "Capacidad
  espacial: Un solo arrendajo esconde entre 3.000 y 5.000 bellotas por temporada
  en miles de lugares distintos, y las recupera meses después."

The README tells the jay story with the sourced episodic-memory fact and the
banner phrase "Hides thousands of acorns. Remembers where, months later."
Before the figure "3.000 a 5.000 bellotas" appears in public copy, cite a
source for it.

## Chosen identity

Direction 03, **hexagon-facet jay with an acorn** (`design/qbit-jay-identity/03-hexagon-facets.html`):
a blue jay built from triangle facets, holding an acorn, inside a flat-top
hexagon. Tokens are in `design/system.md`; the other two directions (01, 02)
stay as alternatives.

| Asset (`assets/branding/`) | Intended use |
| --- | --- |
| `qbit-jay-mascot.svg`, `qbit-jay-mascot-512.png` | Full jay with acorn, transparent, no glow: docs, TUI splash source, web empty states |
| `qbit-jay-isotype-dark.svg`, `qbit-jay-isotype-light.svg`, `qbit-jay-isotype-dark-256.png` | Hexagon tile with the jay head: favicon, avatar, small marks. Dark = for dark backgrounds, light = for light backgrounds |
| `qbit-jay-logo-horizontal-dark.svg`, `qbit-jay-logo-horizontal-light.svg` | Isotype, wordmark `qbit-jay` and `by qbitdev`; transparent background |
| `qbit-jay-banner.svg`, `qbit-jay-banner.png` (1280x320), `qbit-jay-banner@2x.png` (2560x640) | README header, with the tagline "Hides thousands of acorns. Remembers where, months later." |
| `qbit-jay-banner.gif` | Animated README banner, 900x506, 20 fps, 7 s, 140 frames, looping, about 0.8 MB: the jay appears in its glowing hexagon, hides acorns in grid cells labelled `save`, `search`, `context`, `sync`, `recall`, `remember`, the wordmark is written on, then the tagline appears. Suggested alt text: "qbit-jay faceted blue jay hides acorns in a hexagon grid, the qbit-jay wordmark is written on, and the tagline Hides thousands of acorns. Remembers where, months later. appears" |

All SVGs are self-contained, use only `design/system.md` colours and have the
wordmark and banner text converted to paths (Outfit), so they need no font.

- **Clear space** (qbit-jay decision): half the isotype height on every side.
- **Minimum size** (qbit-jay decision): isotype 32 px.
- **Fonts**: Outfit (wordmark and headlines), Inter (body), JetBrains Mono
  (labels), all SIL OFL 1.1, taken from `qbitdev-brand/fonts`.
- Engram's images are removed from `assets/branding/` and `plugin/pi/assets/`; a test (`TestNoUpstreamBrandAssetsAreShipped`) fails if any returns.

## Rules

- A fork must use its own name and identity (`TRADEMARKS.md`). "Based on
  Engram" and "compatible with Engram" are allowed when true; Engram logos and
  confusingly similar names are not.
- Do not imitate Engram's identity: the TUI and web read the qbit-jay tokens
  from `internal/qbit/theme`.

## Where identity lives today

| Surface | Location | What it holds |
| --- | --- | --- |
| Images | `assets/branding/` | `qbit-jay-*` final assets; no Engram image remains |
| Screenshots | `assets/screenshots/` | TUI screens rendered by `internal/tui` (`TestRenderPreviewScreens`) with sample data |
| TUI theme | `internal/tui/styles.go` | Lip Gloss colours read from `internal/qbit/theme` |
| Web theme | `internal/cloud/dashboard/static/styles.css` | Dashboard CSS on `--qj-*` variables generated from `internal/qbit/theme` |
| Cloud branding | `docs/engram-cloud/branding.md` | Points to this document and `design/system.md` |

## Single source of theme tokens

`internal/qbit/theme` holds the tokens for light and dark themes: colours,
typography and spacing. The TUI reads them as Lip Gloss styles and the web
interface as CSS variables, so a change to the identity is a change to that one
package (checked against `design/system.md` by tests).

| Element | Plugs into | State |
| --- | --- | --- |
| Logo | `assets/branding/` and the README header | Applied |
| Palette | `internal/qbit/theme` | Applied (`design/system.md`) |
| Typography | `internal/qbit/theme`, embedded dashboard fonts | Applied (Outfit, Inter, JetBrains Mono) |
