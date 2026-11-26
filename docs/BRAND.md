# Brand plug-in points

The visual identity of qbit-jay (jay logo, palette, typography, TUI and web
style) is **not decided yet**. The user will define it in a separate design
session. This document records where that identity plugs in, so it can be
applied without searching the codebase.

Status: inventory at upstream commit `11521f4`. Nothing has been renamed or
replaced yet; that work belongs to task T0.4.

## Rules

- A fork must use its own name and identity (`TRADEMARKS.md`). "Based on
  Engram" and "compatible with Engram" are allowed when true; Engram logos and
  confusingly similar names are not.
- Until the design exists, use a neutral provisional theme and a logo
  placeholder. Do not invent the final identity and do not imitate Engram's.

## Where identity lives today

| Surface | Location | What it holds |
| --- | --- | --- |
| Images | `assets/branding/` | Engram banner, logo and Cloud elephant images: to be removed |
| Screenshots | `assets/*.png` | TUI and agent screenshots showing Engram branding: to be retaken |
| TUI theme | `internal/tui/styles.go` | Lip Gloss colours (Catppuccin Mocha) |
| Web theme | `internal/cloud/dashboard/static/styles.css` | Dashboard CSS on top of `pico.min.css` |
| Cloud branding notes | `docs/ENGRAM-CLOUD-BRANDING.md` | Upstream's branding guidance for the Cloud dashboard |

## Planned single source of theme tokens

`internal/qbit/theme` (to be created in T0.4) will hold the tokens for light and
dark themes: colours, typography and spacing. The TUI reads them as Lip Gloss
styles and the web interface as CSS variables. Replacing the provisional theme
then means editing that one package.

| Element | Plugs into | State |
| --- | --- | --- |
| Logo | `assets/branding/` and the README header | Pending design |
| Palette | `internal/qbit/theme` | Pending design |
| Typography | `internal/qbit/theme` | Pending design |
