# qbit-jay — visual identity

Visual identity of qbit-jay: cartoon blue-jay mascot, logo and wordmark, palette and typography, README banner.
Traceability: no business contract (`bcdd/contrato.md`) exists; this implements section 8.4 of
`docs/qbit-jay — Especificación para el agente de código.md` ("El diseño ... lo decidirá el usuario en otra sesión") and the plug-in points in `docs/BRAND.md`. Path: `odd/tasks/qbit-jay-identity.md`.

## Specs

- **S1** (user, L1) "este es el pajaro y necesito que generes la identidad de la herramienta en base a esta imagen y el branding de qbitdev. debe ser en dibujos animado parecido a engram pero con la imagen de la foto"
- **S2** (user, L2) Deliverables: "Mascota cartoon del arrendajo", "Logo y wordmark", "Paleta y tipografía (design/system.md)", "Banner para el README".
- **S3** (user, L2) qbitdev branding source: "Sí, leé qbitdev-brand" (`D:\Proyectos\Propios\qbitdev-brand`, read only).
- **S4** (user, L3, brand story) "Capacidad espacial: Un solo arrendajo esconde entre 3.000 y 5.000 bellotas por temporada en miles de lugares distintos, y las recupera meses después."
- **S6** (user, L11) "lo que no generaste es un gif asi como tiene engram"
- **S5** (spec 6.2 and `TRADEMARKS.md` via `docs/BRAND.md`) A fork uses its own name and identity; Engram logos and confusingly similar identity are not allowed.

Assumptions (not user specs):

- A1: "parecido a engram" means the same genre (friendly cartoon mascot, glowing outline on a dark background, one saturated accent), not Engram's drawing, animal, pose or magenta palette.
- A2: no image generation model is available in this session (`models.getAvailableOfType("image")` is empty), so the mascot is a hand-built SVG vector, which is also what the README, TUI and web need.
- A3: the jay is a product character of the qbitdev family, separate from bit0 (the qbitdev AI-agent mascot); they never appear together in one piece ("un personaje por pieza").
- A4: the reference photo stays local and untracked (`design/reference-blue-jay.png`, excluded via `.git/info/exclude`) because its licence is unknown; the repository only holds our own drawings.
- A5: the palette reuses qbitdev tokens; jay-specific colours (crest blue, mask black, belly grey) are additions recorded as qbit-jay tokens, never replacing qbitdev ones.

## Tasks

- [x] T1 Design system: `design/system.md` and `design/system.html` from qbitdev `tokens.css`/`tokens.json` plus qbit-jay additions — S2, S3, S5 — worker — commit: pending
- [x] T2 Three identity directions in `design/qbit-jay-identity/` (mascot, logo and wordmark, README banner per direction) — S1, S2, S4, S5 — worker — commit: pending
- [x] T3 User chooses a direction — human. Chosen: `03-hexagon-facets.html` (L9); 01 and 02 are alternatives
  - [x] T2a Fix the black head marking of `01-neon-outline.html` to match the photo — S1 — worker
- [x] T4 Final assets in `assets/branding/` from `03-hexagon-facets.html` (standalone SVG mascot, isotype, lockups dark/light, README banner; PNG exports); `docs/BRAND.md` updated; Engram assets and README header stay until T0.4 — S1, S2, S5 — worker
- [x] T5 Animated README banner `assets/branding/qbit-jay-banner.gif` like Engram's (`engram-banner.gif`: 900x506, 20 fps, 7 s, 140 frames, 2.4 MB) — S1, S6 — worker

## Log



- L1 (user, 2026-11-27): "C:\Users\QuBit\AppData\Local\Temp\orca-paste-1795814508773-bf802bc3-fce1-4637-b4dd-8d575d1154a1.png este es el pajaro y necesito que generes la identidad de la herramienta en base a esta imagen y el branding de qbitdev. debe ser en dibujos animado parecido a engram pero con la imagen de la foto"
- L2 (user, 2026-11-27): answers: qbitdev-brand "Sí, leé qbitdev-brand"; deliverables "Mascota cartoon del arrendajo, Logo y wordmark, Paleta y tipografía (design/system.md), Banner para el README".
- L3 (user, 2026-11-27, earlier): jay story, S4, recorded in `docs/BRAND.md`.
- L4 (evidence, 2026-11-27): reference photo is a blue jay (*Cyanocitta cristata*): blue crest, white face with black eye line and black necklace, blue wings with white bars and black barring, long blue tail, grey-white belly. qbitdev brand (`qbitdev-brand/tokens.css`, `brand.md`, `logo/vector/`): dark bg `#04070D`, panel `#0B121B`, blue `#2D7FFF`, cyan `#35E0FF`, green `#3DFF8B`, ink `#E8F2FF`; gradient blue→cyan→green; Outfit (display), Inter (body), JetBrains Mono; hexagon isotype with orbit; one strong glow per piece. Engram's mascot: soft cartoon elephant, dark grey with hot magenta neon rim on black.
- L5 (evidence, 2026-11-27): T1 and T2 written by a worker: `design/system.md`, `design/system.html`, `design/qbit-jay-identity/` with `01-neon-outline.html`, `02-flat-sticker.html`, `03-hexagon-facets.html` and `design.json`; checked with Playwright at 1440 px. Banner copy "Hides thousands of acorns. Remembers where, months later." (figure left out until sourced). Generators live only in the temp dir.
- L6 (user, 2026-11-27): "me gusta el tercer diseño o el primero, aunque el primero la linea negra de la cabeza esta mal"
- L7 (user, 2026-11-27): "algo que no se si te dije es que aprovechemos que hoy engram esta maduro y bien diseñado. nos apalanquemos y lo mejoremos aun mas"
- L9 (user, 2026-11-27): "me quedo con el 3." Chosen artboard: `design/qbit-jay-identity/03-hexagon-facets.html`; 01 and 02 are alternatives. The user accepted the hexagon echo of the qbitdev isotype.
- L8 (evidence, 2026-11-27): T2a done by a worker: 01 now has a 4.5 px mask line from beak through eye to nape and an 11 px necklace across the upper breast below the white throat, shared by every mascot instance in 01. Parent screenshot at 1440 px confirms it. 02, 03 and system.* untouched.
- L10 (evidence, 2026-11-27): T4 done by a worker: `assets/branding/qbit-jay-*` (mascot, isotype dark/light, horizontal logo dark/light, banner as SVG; mascot 512, isotype 256, banner 1x/2x as PNG), wordmark and tagline outlined from Outfit (OFL, qbitdev-brand), no external refs or rasters in the SVGs; `docs/BRAND.md` has a "Chosen identity" section. Parent viewed banner, mascot, both logos and both isotypes at 32 px. Engram assets and README untouched until T0.4.
- L11 (user, 2026-11-27): "si esta bien.. lo que no generaste es un gif asi como tiene engram" (light variants keep the dark tile; banner size accepted). Engram's README uses `assets/branding/engram-banner.gif`, 900x506, 20 fps, 7 s: the elephant exhales a memory stream (remember, sync, recall), the wordmark is written on, then the tagline appears.
- L12 (evidence, 2026-11-27): T5 done by a worker: `assets/branding/qbit-jay-banner.gif`, 900x506, 20 fps, 140 frames, 7 s, infinite loop, 821,823 bytes. Story: jay appears in its glowing hexagon, hides six acorns in labelled cells (save, search, context, sync, recall, remember), wordmark written on, tagline, fade to empty grid. Parent extracted frames 35, 60, 90 and 115 from the final GIF and checked them. Unlike the static banner, the animated wordmark colours "jay" green.
