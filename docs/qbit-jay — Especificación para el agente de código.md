# qbit-jay — Especificación para el agente de código

Nov 26, 2026 · @Leandro Álvarez

## 0. Cómo usar este documento

qbit-jay es un fork de Engram con otro nombre e identidad, 100 % compatible y con mejoras medidas. Este documento es la especificación completa para empezar a construirlo. Léelo entero antes de tocar código.

**Orden de lectura recomendado:** secciones 2 (principios), 5 (decisiones del usuario), 6 (estrategia de fork) y 11 (plan de trabajo). El resto es referencia.

**Reglas de oro para el agente:**

1. **Paridad antes que mejoras.** Nada de mejoras hasta que el fork renombrado pase toda la suite de tests de upstream y los benchmarks existentes (Fase 0).
2. **Cada mejora se justifica con datos.** Si no pasa los gates de la sección 2, no entra. No se aceptan cambios "porque parece mejor".
3. **Cero fricción para el usuario.** Ningún paso nuevo, ninguna configuración obligatoria, ninguna API key.
4. **Compatibilidad total** con agentes, gentle-ai y la metodología ODD (sección 7). Los nombres de herramientas `mem_*` y la forma de sus respuestas no cambian.
5. **Fork delgado.** Las mejoras viven en paquetes propios y tocan el núcleo de upstream en el menor número de puntos posible.
6. **El diseño visual (TUI, web, logo, paleta) NO se decide aquí.** Lo definirá el usuario en otra sesión. Deja los puntos de tema y marca preparados para recibirlo (sección 8.4).
7. **Ante la duda, pregunta.** Las preguntas que había ya están resueltas (sección 12.2). Si surge una nueva que cambia el comportamiento visible, no la resuelvas por tu cuenta.
8. **Trabaja con ODD.** Este proyecto se desarrolla con la propia metodología ODD de gentle-ai: documento de tareas en `odd/tasks/` y su espejo en memoria.

**Fuentes de verdad:** el repositorio de upstream ([Gentleman-Programming/engram](https://github.com/Gentleman-Programming/engram), commit `11521f4` del 24-11-2026 usado en este análisis), el repositorio de [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai) (commit `423d90a` del 26-11-2026) y la auditoría de la base de datos real del usuario (sección 4). Si algo de este documento contradice el código actual de upstream, gana el código: verifica y avisa.

## 1. Contexto y objetivo

**Objetivo:** construir qbit-jay, la herramienta de memoria persistente para agentes de código de la identidad **qbitdev**. Debe conservar todo lo que Engram ya hace y ser mejor en relevancia, búsqueda y rendimiento, sin añadir fricción.

**Por qué un fork y no una herramienta desde cero:**

- Engram es un proyecto muy maduro: unas 300.000 líneas de Go contando los tests, con `internal/store/store.go` de más de 15.000 líneas. En el último mes hubo al menos 50 commits.
- Tiene tests de paridad entre transportes (MCP, HTTP, plugin Pi) para cada herramienta, contratos de comportamiento de agentes y benchmarks.
- Rehacerlo costaría meses y perdería calidad probada. Su licencia MIT permite el fork; su política de marca obliga a usar otro nombre e identidad (sección 6).

**Nombre:** `qbit-jay`, por el arrendajo (*jay*). Los arrendajos de matorral fueron el primer animal en el que se demostró memoria "episódica": recuerdan **qué** escondieron, **dónde** y **cuándo** (Clayton y Dickinson, *Nature*, 1998). Esa historia coincide con lo que diferencia al producto:

- **qué** = búsqueda de calidad, también entre idiomas;
- **dónde** = contexto según el proyecto, la rama y los archivos en los que trabajas;
- **cuándo** = saber qué memorias siguen vigentes y cuáles caducaron.

**Usuario y entorno de referencia:**

- Windows con WSL. Los datos de memoria viven dentro de WSL (`~/.engram` hoy). Los proyectos están en `/mnt/c/PROYECTOS/...`.
- Usa el ecosistema gentle-ai (que instala y configura Engram), la metodología ODD y varios agentes: Pi (gentle-pi), Codex, Claude Code y orca, entre otros. Hay orquestador y subagentes.
- 21 proyectos activos en memoria. El principal (`web-ai-mate`) concentra el 65 % de las memorias.
- Su idioma de trabajo con los agentes es el español; las memorias que guardan los agentes están casi todas en inglés.
- Otras herramientas de su identidad: **qbit-vault** gestionará los secretos (por eso qbit-jay no implementa detección de secretos; sección 5), y qbit-ai, un ecosistema propio basado en gentle-ai del que qbit-jay será el componente de memoria (sección 7.4).

## 2. Principios y criterios de aceptación

Un cambio entra en qbit-jay solo si cumple **todos** los gates. Las premisas del usuario son: cero fricción, rapidez, lo óptimo, rendimiento y mejores resultados.

**Principios no negociables:**

1. **Cero fricción.** Ningún paso nuevo para el usuario, ninguna configuración obligatoria, ninguna API key ni servicio externo obligatorio. Todo lo nuevo funciona por defecto o es opt-in sin coste para quien no lo usa.
2. **Local primero y binario único.** Se conserva la filosofía de Engram: un binario de Go y un archivo SQLite, sin Node, Python ni Docker para el uso normal. El build es CGO-free (usa `modernc.org/sqlite`); no se introduce CGO sin decisión explícita del usuario.
3. **El agente decide qué guardar.** Engram eligió a propósito no capturarlo todo (ver `docs/COMPARISON.md` de upstream): menos ruido, sin llamadas extra a un LLM. qbit-jay mantiene esa decisión.
4. **Compatibilidad total** con las herramientas `mem_*`, la CLI, los hooks, gentle-ai y ODD (secciones 7 y 8).
5. **Fork delgado** y sincronizado con upstream (sección 6).
6. **Medir antes de decidir.** Las hipótesis sin datos van a fases posteriores con criterio de descarte.

**Gates de aceptación de cada cambio** (los números son valores iniciales; ajústalos con el usuario tras medir la línea base en la Fase 0):

| Gate | Criterio |
| --- | --- |
| Tests | Pasa la suite completa de upstream, incluida `go test -race`, más los tests nuevos del cambio |
| Latencia | p95 de `mem_search`, `mem_save` y `mem_context` no empeora más de un 5 % frente a la línea base |
| Arranque | El tiempo de arranque del MCP (stdio) no empeora más de un 5 % |
| Tamaño | El binario no crece más de un 10 % (sin contar componentes opcionales descargables) |
| Tokens | Los tokens que inyecta `mem_context` en una sesión típica no aumentan |
| Calidad | Mejora medible en el evaluador (sección 10) o, si es un arreglo, un test que reproduce el fallo |
| Fricción | Cero pasos nuevos para el usuario y cero configuración obligatoria |
| Upstream | Toca el núcleo de upstream en pocos puntos, documentados en `UPSTREAM-DELTA.md` |

**Criterio de descarte:** si una mejora no demuestra beneficio en el evaluador tras su experimento, se retira y se documenta el resultado. No se mantiene código especulativo.

## 3. Estado actual de Engram (verificado en el código)

Todo lo de esta sección se leyó en el commit `11521f4` de upstream. qbit-jay debe conservarlo íntegro.

### 3.1 Stack y estructura

| Elemento | Valor |
| --- | --- |
| Módulo Go | `github.com/Gentleman-Programming/engram/v3` |
| Toolchain | `go 1.26.11` (directiva de `go.mod`; con Go 1.24 no compila) |
| SQLite | `modernc.org/sqlite` v1.58 (Go puro, sin CGO), modo WAL |
| MCP | `github.com/mark3labs/mcp-go` v0.44 |
| TUI | Bubble Tea, Bubbles, Lip Gloss (Charmbracelet); tema Catppuccin Mocha |
| Dashboard web (Cloud) | `a-h/templ` + htmx (skill `dashboard-htmx`) |
| Cloud | Postgres vía `jackc/pgx/v5` |
| Release | GoReleaser (`.goreleaser.yaml`), Homebrew tap, firma SignPath en Windows |
| Make | `templ`, `lint`, `perf-check`, `perf-baseline`, `deadcode-check`, `deadcode-baseline` |

Paquetes en `internal/`: `agentops`, `cloud` (`cloudserver`, `cloudstore`, `dashboard`, `autosync`), `cloudconfig`, `command`, `daemon`, `diagnostic`, `hostbind`, `identityfile`, `llm`, `mcp`, `obsidian`, `project`, `selftest`, `server`, `setup`, `store`, `sync`, `timeutil`, `tui`, `version`. Plugins en `plugin/`: `claude-code`, `codex`, `opencode`, `pi` (paquete `gentle-engram`), `obsidian`.

### 3.2 Superficies que hay que conservar

- **CLI** (`cmd/engram`), subcomandos: `alias`, `claude-code`, `cloud`, `conflicts`, `consolidate`, `context`, `delete`, `doctor`, `export`, `hook`, `import`, `init`, `logs`, `mcp`, `merge`, `obsidian-export`, `opencode`, `pi`, `project`, `projects`, `prompt`, `protocol-mode`, `prune`, `ps`, `rescue-ownership`, `restart`, `run`, `save`, `search`, `serve`, `session`, `setup`, `stats`, `stop`, `sync`, `timeline`, `tui`.
- **MCP** (stdio, con perfiles de herramientas: gentle-ai lo lanza como `mcp --tools=agent`). Herramientas registradas: `mem_capture_passive`, `mem_compare`, `mem_context`, `mem_current_project`, `mem_delete`, `mem_doctor`, `mem_get_observation`, `mem_judge`, `mem_judge_many`, `mem_list_projects`, `mem_merge_projects`, `mem_pin`, `mem_review`, `mem_save`, `mem_save_prompt`, `mem_search`, `mem_search_prompts`, `mem_session_end`, `mem_session_start`, `mem_session_summary`, `mem_stats`, `mem_suggest_topic_key`, `mem_timeline`, `mem_unpin`, `mem_update`. La documentación habla de 27; verifica el inventario con `TestAgentContractInventoryAndMCPParity` (también `mem_consolidate` y `mem_session_finalize` aparecen en `docs/AGENT-BEHAVIOR-EVIDENCE.md`).
- **API HTTP** (`engram serve`, daemon, socket), documentada en `DOCS.md`.
- **TUI** (`engram tui`): navegar, buscar y copiar observaciones.
- **Hooks y plugins** por agente: Claude Code (SessionStart, SubagentStop, fin de sesión, compactación), Codex, OpenCode, Pi.
- **Setup** (`engram setup <agente>`) para 14 destinos: Claude Code, Codex, Cursor, Gemini CLI, Antigravity, OpenCode, VS Code Copilot, Pi, entre otros.
- **Git Sync** (chunks comprimidos), **Cloud** (replicación por proyecto, dashboard, autosync, audit log), **export a Obsidian** (beta), **doctor**, alias y fusión de proyectos.
- **Instrucciones de protocolo** que se inyectan al agente (`MEMORY_INSTRUCTIONS`, skills `memory-protocol`) y el contrato `docs/contracts/agent-contract-v1.json`.

### 3.3 Modelo de datos

Tablas principales: `observations`, `observation_versions` (contenido completo de cada revisión), `sessions`, `user_prompts`, `memory_relations`, `observations_fts` y `prompts_fts`, `capture_project_provenance`, `observation_save_operations`, `session_finalizations`, `project_aliases` y las tablas de sync (`sync_*`, `cloud_upgrade_state`).

Columnas de `observations`: `id`, `sync_id`, `session_id`, `type`, `title`, `content`, `tool_name`, `project`, `scope` (`project` / `personal` / `global`), `topic_key`, `normalized_hash`, `revision_count`, `duplicate_count`, `last_seen_at`, `pinned`, `created_at`, `updated_at`, `deleted_at`, `review_after`, `expires_at`, `embedding`, `embedding_model`, `embedding_created_at`, `superseded_by_sync_id`, `org`. **Las columnas de embedding están reservadas y sin uso**: upstream parece preparar búsqueda semántica.

### 3.4 Búsqueda y ranking

- FTS5 con `tokenize='trigram remove_diacritics 1'` sobre `title`, `content`, `tool_name`, `type`, `project`, `topic_key`.
- `sanitizeFTS` entrecomilla cada palabra. `match_mode` `all` (por defecto, AND) o `any` (OR).
- Si la consulta contiene `/`, primero busca por `topic_key` exacto.
- Ranking BM25 con pesos: título 5, contenido 1, `topic_key` 3. Boosts multiplicativos acotados: fijada 0,10; recencia 0,06 (media puntuación a 30 días); estabilidad 0,04.
- Resultados como previews; el agente pide el contenido completo con `mem_get_observation`. Opción `response_format=compact`.
- Benchmarks existentes (`internal/store/search_bench_test.go` y otros): miden **latencia**, no calidad de recuperación.

### 3.5 Contexto, límites y ciclo de vida

- `FormatContext` arma el contexto **solo por recencia**: 5 sesiones recientes (resumen truncado a 200 caracteres), 10 prompts recientes (200), memorias fijadas, 20 observaciones recientes no fijadas (preview de 300 caracteres o compacto) y un historial antiguo con presupuesto de 2.048 bytes.
- `mem_context`: `max_bytes` por defecto 16.384, máximo 65.536 (unos 4.000 tokens por defecto).
- `MaxObservationLength` = **50.000** caracteres: lo que exceda se corta al final con `... [truncated]`.
- `MaxContextResults` = 20.
- Revisión periódica (`review_after`) por tipo: decisión 6 meses, política 12, preferencia 3, config 3, discovery 6, pattern 6, architecture 12, bugfix 12. No borra nada.
- Privacidad: elimina `<private>...</private>` en dos capas antes de guardar. No hay detección automática de secretos.
- Duplicados: `normalized_hash`, `duplicate_count` y upsert por `topic_key`. `mem_consolidate` fusiona con vista previa y confirmación.
- Conflictos: `mem_compare`, `mem_judge` y `mem_judge_many`; el juicio semántico usa `claude -p --model haiku` u OpenCode como runner (`internal/llm`).
- Captura pasiva por host: SubagentStop en Claude, salidas de tareas en OpenCode, captura explícita en Pi y `mem_capture_passive` genérico.
- Rechaza directorios de datos conocidos en NFS o SMB/CIFS; un sistema de archivos desconocido se acepta, pero upstream no lo garantiza como local. En WSL, la base debe vivir en el sistema de archivos de Linux (como hoy, `~/.engram`) y no en `/mnt/c`, donde SQLite en modo WAL no es fiable.

### 3.6 Variables de entorno relevantes

`ENGRAM_DATA_DIR`, `ENGRAM_PORT`, `ENGRAM_URL`, `ENGRAM_SOCKET`, `ENGRAM_PROJECT`, `ENGRAM_TOKEN`, `ENGRAM_HTTP_TOKEN`, `ENGRAM_TIMEZONE`, `ENGRAM_NO_UPDATE_CHECK`, `ENGRAM_BIN`, `ENGRAM_AGENT_CLI`, `ENGRAM_HOOK_MAX_TIME`, `ENGRAM_HOOK_OUTPUT`, `ENGRAM_HOOK_CODEX_COMPACTION`, `ENGRAM_NUDGE_COOLDOWN_SECS`, `ENGRAM_DATABASE_URL`, `ENGRAM_JWT_SECRET`, `ENGRAM_CLOUD_*` (servidor, token, autosync, proyectos permitidos, admin, etc.). Las de prueba (`ENGRAM_TEST_*`, `*_CHILD`) son internas.

### 3.7 Gobierno del repositorio

- `AGENTS.md` y un catálogo de skills internas en `skills/`: `architecture-guardrails`, `business-rules`, `gentleman-bubbletea`, `tui-quality`, `visual-language`, `ui-elements`, `dashboard-htmx`, `server-api`, `plugin-thin`, `testing-coverage`, `memory-protocol`, `commit-hygiene`, `branch-pr`, `pr-review-deep`, entre otras. **Úsalas**: describen las reglas con las que está hecho el código.
- `CONTRIBUTING.md`: cualquier cambio en `plugin/claude-code/` exige subir la versión en `plugin.json` y en `.claude-plugin/marketplace.json`.
- CI extensa en `.github/workflows/` (ci, release, publish-pi, cloud-image, signpath, políticas de issues y PR). Muchas dependen de infraestructura de upstream y habrá que adaptarlas (sección 6).
- Documentación en `docs/` (`ARCHITECTURE.md`, `CODEBASE-GUIDE.md`, `AGENT-CONTRACT.md`, `RUNTIME-CONTRACT.md`, `TEAM-USAGE.md`, `ENGRAM-CLOUD.md`, `INSTALLATION.md`, etc.).

## 4. Auditoría de la memoria real del usuario

Se analizó en solo lectura una copia del `engram.db` del usuario (integridad `ok`): 2.387 observaciones (2.290 vivas, 97 borradas), 2.637 sesiones y 2.694 prompts, del 01-10-2026 al 26-11-2026. Los scripts y el método están en el apéndice (sección 13).

### 4.1 Lo que ya funciona bien (no tocar)

- **Duplicados:** solo 4 pares de memorias casi idénticas (Jaccard ≥ 0,5 sobre trigramas de palabras, sin contar resúmenes). La deduplicación de upstream funciona.
- **Temas estables:** el 79 % de las memorias (1.814) usa `topic_key`; 402 se han revisado más de una vez en lugar de duplicarse.
- **Coste de contexto:** con la configuración por defecto el contexto inicial ronda los 3.000 tokens (tope de 16 KB, unos 4.000). No es un problema de coste; sí de relevancia (H4, H13).

### 4.2 Hallazgos

| # | Hallazgo | Dato |
| --- | --- | --- |
| H1 | Los espejos ODD (`odd/*/tasks`) son pocos pero muy pesados | 139 memorias (6 %) = 28 % del contenido (1,15 M de 4,0 M de caracteres) y 92 % del historial de versiones (52 de 56 MB) |
| H2 | Los espejos ODD que pasan de 50.000 caracteres se truncan por el final | 10 memorias truncadas, 9 son espejos ODD; se pierde la parte más reciente del log de progreso |
| H3 | Cada revisión guarda el contenido completo | Un espejo se reescribió 342 veces (unos 50 KB cada vez); la base ocupa unos 90 MB más 90 MB de WAL |
| H4 | El contexto inicial solo usa recencia | En 39 sesiones evaluables, las 20 memorias más recientes contenían la memoria que la sesión necesitó en 17 (44 %); añadiendo las memorias cuyo tema menciona el prompt, en 24 (62 %) |
| H5 | Idiomas distintos entre consultas y memorias | El 62 % de los prompts están en español; el 91 % de las memorias, en inglés (muestras de 400) |
| H6 | No hay registro de búsquedas | Engram no guarda qué se busca ni qué se abre: no se puede medir la calidad de búsqueda en el uso real |
| H7 | Muchas memorias mencionan archivos | 1.005 de 1.952 memorias que no son resúmenes (51 %) contienen rutas de archivo |
| H8 | Resúmenes de sesión con título genérico | 331 de 338 se titulan "Session summary" (el título pesa 5 veces más en el ranking) |
| H9 | Relaciones y conflictos casi sin uso | 13 relaciones en dos meses, 7 pendientes de juzgar |
| H10 | La mayoría de las sesiones están vacías | De 2.637 sesiones, 290 tienen memorias, 335 solo prompts y 2.012 (76 %) no tienen ni memorias ni prompts. Probablemente las registran hooks o subagentes (no verificado) |
| H11 | Secretos | Las memorias están limpias. En los prompts capturados aparecen 1 API key y 3 JWT. Un detector ingenuo dio unos 2.100 resultados, casi todos falsos positivos |
| H12 | Los prompts capturados se cortan | Los plugins de Pi y OpenCode truncan el prompt capturado a 2.000 caracteres (`truncate(..., 2000)`); 223 prompts miden exactamente 2.003 |
| H13 | El bloque "Recent Sessions" del contexto se llena de sesiones vacías | La consulta usa `LEFT JOIN` e incluye sesiones sin memorias ni resumen. En `web-ai-mate`, el 68 % de esas entradas estaban vacías (simulación en 200 inicios de sesión); hoy las 5 están vacías en 3 de los 4 proyectos principales |

### 4.3 Limitaciones del análisis

- La evaluación de contexto (H4) usa solo 39 sesiones, casi todas de subagentes con prompts de delegación en inglés. Es un indicio, no una prueba.
- La relevancia se aproximó así: "memoria que ya existía y que se revisó durante la sesión". No captura lo que el agente leyó sin modificar.
- Sin registro de búsquedas (H6) no se pudo evaluar la búsqueda real. La prueba con palabras clave extraídas de los prompts no es representativa de cómo busca un agente, así que sus cifras no se usan para decidir.
- La detección de idioma se hizo con palabras funcionales del español: es aproximada.
- Solo dos meses de datos y un único usuario.

### 4.4 Qué implica

Las mejoras con evidencia son H2 y H3 (espejos ODD), H4 y H13 (contexto) y H6 (telemetría). H5 y H7 justifican experimentos. H1 no es un problema en sí: los espejos ODD son parte del diseño de gentle-ai (sección 7). H8, H9 y H11 quedan fuera de alcance por decisión del usuario o por falta de valor (sección 5). H10 y H12 se documentan; H10 se trata dentro de M3 vía H13 y H12 se mantiene como está por decisión del usuario (Q12).

## 5. Decisiones del usuario

Estas decisiones las tomó el usuario el 26-11-2026 a partir de la auditoría. No se reabren sin preguntárselo.

| Tema | Decisión | Motivo |
| --- | --- | --- |
| Paridad con Engram | **Entra entero**: CLI, TUI, MCP, HTTP, hooks, plugins, setup, sync, Cloud, Obsidian, doctor | "Debe tener todo lo de Engram" |
| Interfaz web local | **Entra**: pequeña, bonita, servida por el propio binario | Pedido explícito. El diseño visual se decide en otra sesión con otro agente |
| ODD y espejos `odd/*/tasks` | **Se mantienen tal cual**. Solo se arregla lo que rompe su contrato (truncado) y la eficiencia interna | ODD es la metodología de gentle-ai y debe seguir funcionando junto a la memoria |
| Resúmenes de sesión | **No se cambian**: se siguen guardando igual, con su título actual | "Es el resumen de la sesión y no me parece malo que se vaya guardando" |
| Contexto por relevancia (H4) | **Entra** en la Fase 1 | "Esto sí deberíamos mejorarlo" |
| Detección de secretos | **Fuera de alcance** | No guardar secretos es una regla de seguridad de su entorno; los secretos los gestionará **qbit-vault** |
| Búsqueda semántica e interlingüe (H5) | **Entra** como Fase 2, decidida con datos de telemetría | "Esto se podría agregar" |
| Asimilación y aprendizaje "como un humano" | **Entra como experimento** (Fase 3) con criterio de descarte | El usuario no está seguro de que sea útil: hay que demostrarlo |
| Telemetría local de búsquedas (H6) | **Entra** en la Fase 0 | Sin ella no se pueden medir las demás mejoras |
| Aviso de memorias obsoletas (H7) | **Entra en la Fase 2 (Q5)** | Confirmado por el usuario (Q5); hay base de datos para evaluarla |

**Descartado con datos** (no implementar): mejorar la deduplicación (ya funciona, sección 4.1), invertir en relaciones y conflictos (H9), captura automática de todo (contradice el diseño de upstream y añade ruido y tokens), reducir el número de herramientas MCP (upstream ya tiene perfiles como `--tools=agent`).

**Nota informativa para el usuario (no es una tarea):** aunque las memorias están limpias, Engram captura los prompts del usuario y entre ellos aparecieron 1 API key y 3 JWT (H11). Si la regla es "nada de secretos en memoria", conviene revisarlo cuando exista qbit-vault (por ejemplo, una integración en la que qbit-jay no capture prompts que qbit-vault marque como sensibles).

## 6. Estrategia de fork, marca y compatibilidad

qbit-jay es un fork delgado: conserva todo el código de upstream, cambia la identidad visible y añade las mejoras en paquetes propios.

### 6.1 Repositorio y ramas

- Repositorio: `qbitdev/qbit-jay` en GitHub, público (Q1).
- Importar el historial completo de upstream (no un squash) para que las fusiones futuras sean limpias. Remoto `upstream` = `https://github.com/Gentleman-Programming/engram`.
- `main` = qbit-jay. Sincronización con upstream mediante ramas `sync/upstream-AAAA-MM-DD` con **merge** (no rebase), pasando todos los gates antes de integrarlas.
- Cadencia: semanal, o antes si upstream publica una release estable.
- **Script de sincronización** `scripts/sync-upstream.sh` (necesario por el cambio de path del módulo, Q2): trae upstream, fusiona en la rama `sync/...`, reescribe el path `github.com/Gentleman-Programming/engram/v3` → `github.com/qbitdev/qbit-jay` en `.go`, `go.mod`, `.templ`, scripts y configuración de release, regenera templ, ejecuta `go mod tidy` y los tests. La reescritura va en un commit separado. Si un conflicto es solo de imports, se toma la versión de upstream y se vuelve a ejecutar la reescritura.

### 6.2 Marca: qué se renombra y qué no

La política de marca de upstream (`TRADEMARKS.md`) obliga a que un fork use **nombre e identidad propios**. Permite decir con veracidad "basado en Engram" o "compatible con Engram", sin sugerir que es oficial. No se pueden usar los logos de Engram ni nombres confusamente parecidos.

**Se renombra (todo lo que ve el usuario):**

- Binario y comando: `qbit-jay` (carpeta `cmd/qbit-jay`). Textos de ayuda, mensajes y errores.
- Nombre del servidor MCP que escribe el setup: `qbit-jay`.
- Carpeta de datos: `~/.qbit-jay` (Windows: `%USERPROFILE%\.qbit-jay`).
- Variables de entorno: prefijo `QBIT_JAY_*`, **leyendo también `ENGRAM_*` como respaldo** por compatibilidad (la nueva gana si están las dos).
- TUI, interfaz web, README, documentación, `assets/` (fuera el logo y las imágenes de Engram).
- "Engram Cloud" → "qbit-jay Cloud".
- Paquetes y plugins: el paquete Pi `gentle-engram`, el plugin de Claude Code y su entrada en el marketplace, el plugin de OpenCode, el de Codex y el de Obsidian (id del plugin incluido).
- Releases, Homebrew tap (`qbitdev/tap`), imagen Docker de Cloud, paquete npm.
- Comprobación de actualizaciones: `internal/version/check.go` consulta las releases de `Gentleman-Programming/engram` y sugiere el `go install` de upstream. Debe apuntar a las releases de qbitdev.
- Scripts de hooks y plugins que llaman al binario por su nombre: `plugin/claude-code/scripts/*.sh` (`engram hook ...`, `engram serve`), `plugin/codex/hooks`, el plugin de Pi (por defecto ejecuta `engram` del PATH salvo que se defina `ENGRAM_BIN`) y el de OpenCode. En qbit-jay llaman a `qbit-jay`. No se distribuye ningún ejecutable ni alias llamado `engram`: la política de marca protege también ese nombre como identificador de CLI.
- **Path del módulo Go** (decisión Q2): de `github.com/Gentleman-Programming/engram/v3` a `github.com/qbitdev/qbit-jay`. Cambian todos los imports. A favor: identidad completa y `go install github.com/qbitdev/qbit-jay/cmd/qbit-jay@latest` funciona. En contra: cada sincronización con upstream trae conflictos en los imports, que se resuelven de forma mecánica con el script de 6.1.

**No se renombra (para no romper compatibilidad ni multiplicar conflictos con upstream):**

- Los nombres de herramientas `mem_*`, sus parámetros y la forma de sus respuestas.
- El esquema de la base de datos.
- Identificadores internos de Go (paquetes, tipos, funciones).

**Atribución:** mantener la licencia MIT con el copyright original y añadir el de qbitdev. En el README, una línea del tipo "qbit-jay está basado en Engram, de Gentleman Programming" con enlace, sin logos.

### 6.3 Migración y convivencia (cero fricción)

- **Importación automática:** en el primer arranque, si `~/.qbit-jay` no existe y `~/.engram/engram.db` sí, qbit-jay copia la base con la API de backup de SQLite (consistente aunque Engram esté abierto) y aplica sus migraciones. **Nunca modifica la base original.** También existe el comando explícito `qbit-jay import --from-engram [ruta]`.
- **Convivencia:** si Engram y qbit-jay están registrados a la vez en un agente, el agente vería las herramientas `mem_*` duplicadas. `qbit-jay setup <agente>` debe detectarlo, hacer copia de la configuración del agente y sustituir la entrada de Engram por la de qbit-jay, dejando constancia en la salida. Ver la integración con gentle-ai en la sección 7.
- **Reversibilidad:** un comando `qbit-jay setup --restore <agente>` restaura la configuración anterior desde la copia.
- **Puerto y socket:** Engram usa por defecto el puerto HTTP 7437 (`ENGRAM_PORT`) y un socket configurable. qbit-jay mantiene el 7437 por compatibilidad con hooks y plugins, pero `qbit-jay doctor` y el arranque del daemon deben detectar si lo ocupa un proceso de Engram y explicarlo, en lugar de fallar en silencio o hablar con el servidor equivocado.

### 6.4 Arquitectura de las mejoras

- Todo el código nuevo va en `internal/qbit/...` (por ejemplo `internal/qbit/contextrank`, `internal/qbit/telemetry`, `internal/qbit/docstore`, `internal/qbit/semantic`, `internal/qbit/webui`).
- El núcleo de upstream solo recibe **puntos de enganche mínimos**: interfaces pequeñas que por defecto se comportan exactamente como upstream (por ejemplo, un `ContextRanker` cuya implementación por defecto es la recencia actual).
- Zonas calientes donde upstream cambia mucho y los conflictos son más probables: `internal/store/store.go`, `internal/mcp/mcp.go`, `internal/setup/setup.go`, `cmd/engram/main.go`, `internal/server/server.go`. Tocar lo mínimo.
- `UPSTREAM-DELTA.md` en la raíz: lista de cada punto donde qbit-jay modifica código de upstream, con el motivo y el test que lo cubre. Se actualiza en cada PR.
- Lo que sea un arreglo general (por ejemplo, el truncado de espejos ODD de H2) se propone también a upstream, **con permiso del usuario**. Si upstream lo acepta, el delta propio se reduce.

### 6.5 CI, releases y entorno de desarrollo

- Conservar `ci.yml` (tests, race, lint, deadcode, perf-check) y `release.yml` con GoReleaser apuntando a qbitdev.
- **Canales de distribución** (Q7): GitHub Releases con checksums, Homebrew tap `qbitdev/tap`, Scoop (bucket de qbitdev), `go install github.com/qbitdev/qbit-jay/cmd/qbit-jay@latest`, script `curl -fsSL ... | sh` para Linux, macOS y WSL, script de PowerShell (`irm ... | iex`) para Windows y el plugin de Pi en npm con el scope de qbitdev. Los scripts verifican el checksum antes de instalar.
- Desactivar o adaptar lo que depende de upstream: firma SignPath (sin certificado propio, desactivar), políticas de issues y PR, `publish-pi` (al scope npm de qbitdev), `cloud-image` (al registro de qbitdev), CodeRabbit.
- Entorno: Go 1.26.11 en WSL (Linux). Los tests de Windows siguen en CI.
- Respetar la regla de `CONTRIBUTING.md` sobre la versión del plugin de Claude Code.

## 7. Integración con gentle-ai y ODD

qbit-jay debe poder sustituir a Engram dentro de gentle-ai sin que gentle-ai ni ODD noten la diferencia. Esto es un requisito de primer nivel.

### 7.1 Cómo usa gentle-ai a Engram (verificado en su código)

- [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai) es un configurador del ecosistema para 17 agentes. Engram es uno de sus componentes (`ComponentEngram = "engram"`, código en `internal/components/engram/`).
- Lo instala por Homebrew, por descarga de releases de GitHub (Linux y Windows) o con `go install`, y verifica checksums.
- Escribe la configuración MCP de cada agente con el comando `<binario> mcp --tools=agent` (`internal/components/engram/inject.go`).
- Tiene health probe y verificación (`healthprobe.go`, `verify.go`) y referencias a Engram en `opencodeplugin`, `agentguidance` (Pi), `reviewassets`, `shellinstaller` y `filemerge`.
- **Punto de entrada clave:** la variable `ENGRAM_BIN` (`system.EngramBinEnv`). Si apunta a una ruta absoluta de un ejecutable, gentle-ai lo usa tal cual: sin Homebrew, sin descarga, sin `go install` y sin tocar el PATH (`caller_binary.go`).

### 7.2 El contrato de ODD con la memoria

ODD (Organic-Driven Development) es la metodología de gentle-ai. Su contrato está en `internal/assets/skills/_shared/engram-convention.md` y `persistence-contract.md`:

- Para trabajo sustancial autorizado existe un documento local `odd/tasks/{feature}.md` y un **espejo** en memoria con `topic_key` `odd/{feature}/tasks`, en el ámbito del proyecto.
- El espejo contiene **el documento completo** y su ruta en el repositorio, no un resumen.
- `mem_save` con el mismo `topic_key`, proyecto y ámbito es un upsert que sobrescribe. Si se conoce el id, se usa `mem_update`.
- Ambas escrituras (documento y espejo) se **releen** para comprobarlas, porque no son atómicas.
- El espejo se guarda con `capture_prompt: false` cuando la herramienta lo admite.
- El espejo se refresca en cada transición de tarea o cambio de plan y al cerrar.
- Para retomar: contexto de sesión → búsqueda del tema en el proyecto → `mem_get_observation` (contenido completo, porque los resultados de búsqueda vienen truncados) → lectura del documento local → reconciliación.
- Si la memoria no está disponible, el espejo queda "pendiente" y el trabajo sigue. Nunca se guardan secretos en un espejo.

### 7.3 Requisitos para qbit-jay

| # | Requisito | Por qué |
| --- | --- | --- |
| G1 | Aceptar exactamente la misma interfaz de línea de comandos que gentle-ai invoca (`mcp --tools=agent`, `version`, health probe, verify y cualquier otra) | Para funcionar con `ENGRAM_BIN=/ruta/absoluta/qbit-jay` sin cambiar gentle-ai |
| G2 | Mantener la semántica exacta de upsert por `topic_key`, `mem_update`, `mem_get_observation` y el parámetro `capture_prompt` | Es el contrato de ODD |
| G3 | **No truncar en silencio un espejo ODD**: `mem_get_observation` debe devolver exactamente lo que se guardó | El contrato exige el documento completo y su relectura. Hoy los espejos de más de 50.000 caracteres pierden el final (H2) |
| G4 | Guardar las versiones de forma eficiente sin cambiar la API | H1 y H3: el 92 % del historial son copias completas de espejos |
| G5 | Que la búsqueda siga encontrando los espejos por tema y por `/` en la consulta | ODD los localiza así al retomar |
| G6 | Que las herramientas y textos de protocolo que inyecta gentle-ai (que mencionan "Engram") sigan funcionando | Los nombres `mem_*` no cambian; los textos son de gentle-ai |
| G7 | Resolver qué pasa con los hooks y plugins que gentle-ai instala y que ejecutan "engram" por nombre (scripts del plugin de Claude Code, hooks de Codex, paquete Pi gentle-engram, plugin de OpenCode) | ENGRAM\_BIN cubre a gentle-ai y al plugin de Pi, pero no a los scripts que llaman a "engram" del PATH. No se puede distribuir un alias llamado engram (marca). Decidido (Q11): qbit-jay trae sus propios plugins y hooks, qbit-ai los instala de forma nativa y, con gentle-ai, qbit-jay setup sustituye los de Engram (con copia y restore) |

**Tarea de investigación obligatoria (Fase 0):** inventariar cada invocación que gentle-ai hace al binario de Engram (argumentos, salida esperada de `version`, health probe, verify, comprobaciones de actualización) y escribir un test de compatibilidad que ejecute qbit-jay con esas mismas invocaciones. Comprobar también qué hace `eco-update` del usuario y si gentle-ai, al actualizar, puede volver a instalar Engram o reescribir la configuración.

**Modo de integración recomendado:** el usuario define `ENGRAM_BIN` con la ruta absoluta de qbit-jay y gentle-ai lo usa como "su" Engram. En la configuración de los agentes el servidor seguirá llamándose `engram` porque lo escribe gentle-ai; es configuración local del usuario, no marca de qbit-jay. Este modo es la compatibilidad con gentle-ai; el destino es qbit-ai (7.4).

### 7.4 qbit-ai: el ecosistema propio

qbit-ai es el nuevo ecosistema de qbitdev, basado en gentle-ai. qbit-jay será su componente de memoria nativo (decisiones Q8 y Q11).

- **En qbit-ai:** instala qbit-jay por sus canales (Q7), registra el servidor MCP como `qbit-jay` e instala los plugins y hooks propios de qbit-jay, que llaman a `qbit-jay`. Sin `ENGRAM_BIN` ni nombres de Engram.
- **Con gentle-ai (mientras se use):** compatibilidad por `ENGRAM_BIN` más `qbit-jay setup`, que sustituye los plugins de Engram con copia y restore. Los dos modos deben funcionar.
- **Contrato de integración:** qbit-jay documenta en `docs/INTEGRATION.md` todo lo que necesita un configurador: comando MCP y perfiles (`mcp --tools=agent`), formato de `version` y health probe, rutas de datos, variables de entorno, plugins por agente y cómo instalarlos, y la convención ODD de espejos. Se toma como referencia exacta lo que gentle-ai espera hoy de Engram (inventario de T0.6), para que qbit-ai pueda adaptar el componente `engram` de gentle-ai con cambios mínimos.
- **ODD:** qbit-ai hereda la metodología y sus convenciones (`odd/tasks/{feature}.md` y espejo `odd/{feature}/tasks`) salvo que el usuario decida otra cosa.
- qbit-ai en sí queda fuera de este documento; aquí solo se define el contrato que qbit-jay le ofrece.

## 8. Alcance funcional

qbit-jay hace todo lo que hace Engram, con el mismo comportamiento, y añade una interfaz web local.

### 8.1 Paridad: checklist de superficies

Cada punto se da por cumplido cuando sus tests de upstream pasan con el binario renombrado y una prueba manual confirma el comportamiento.

- [ ] CLI completa (todos los subcomandos de la sección 3.2), con `qbit-jay` como nombre
- [ ] Servidor MCP stdio con perfiles de herramientas (`--tools=agent` y los demás)
- [ ] Las herramientas `mem_*` con los mismos parámetros y respuestas
- [ ] API HTTP, daemon, socket y tokens
- [ ] TUI con todas sus funciones
- [ ] Hooks y plugins: Claude Code, Codex, OpenCode, Pi, Obsidian
- [ ] `setup` para los 14 destinos, con sustitución segura de una entrada de Engram existente
- [ ] Git Sync, Cloud (servidor, dashboard, autosync, audit log), export a Obsidian
- [ ] `doctor`, alias y fusión de proyectos, `consolidate`, `conflicts`, `prune`, `rescue-ownership`
- [ ] Instrucciones de protocolo, skills `memory-protocol` y contrato de agente
- [ ] Importación desde `~/.engram` y variables `ENGRAM_*` como respaldo
- [ ] Compatibilidad con gentle-ai vía `ENGRAM_BIN` (sección 7)
- [ ] Windows, macOS y Linux (incluido WSL)

### 8.2 Interfaz web local

Una interfaz pequeña y cuidada para ver y entender la memoria. La sirve el propio binario: sin Node en tiempo de ejecución y sin servicios externos.

**Requisitos técnicos:**

- Comando `qbit-jay ui`: arranca el servidor web y abre el navegador. En WSL, abre el navegador de Windows (`wslview` o `explorer.exe`) y, si no puede, imprime la URL.
- Escucha solo en `127.0.0.1`, puerto libre o configurable, con token de sesión (reutilizar el mecanismo de token HTTP de upstream).
- Assets embebidos con `go:embed`. Stack recomendado: el mismo de upstream para su dashboard (`templ` + htmx, ver la skill `dashboard-htmx`), para no añadir dependencias. Reutilizar lo que ya existe en internal/cloud/dashboard (componentes templ) en lugar de partir de cero. Un SPA solo si el diseño lo exige y con decisión del usuario.
- Usa las mismas funciones del store que MCP y HTTP: nada de lógica duplicada.
- Rendimiento: cada vista responde en menos de 200 ms con 10.000 memorias en una máquina normal.
- Tema claro y oscuro, accesible (contraste, teclado, lectores de pantalla), responsive.

**Vistas:**

| Vista | Qué muestra |
| --- | --- |
| Inicio | Resumen por proyecto: número de memorias, actividad reciente, tipos, sesiones |
| Buscar | Búsqueda con filtros (proyecto, tipo, ámbito, fecha); el mismo motor que `mem_search`; muestra por qué sale cada resultado |
| Memoria | Contenido completo renderizado, metadatos, `topic_key`, historial de versiones con diff, relaciones, fecha de revisión |
| Sesiones y línea de tiempo | Sesiones por proyecto, sus memorias y prompts |
| ODD | Espejos `odd/*/tasks`: estado, última actualización, ruta del documento local, avisos si alguno está incompleto |
| Contexto | Lo que `mem_context` inyectaría ahora para un proyecto (y rama), con su tamaño en tokens y el motivo de cada memoria |
| Telemetría | Búsquedas sin resultado, memorias más usadas, tokens inyectados por sesión (sección 10) |
| Salud | Salida de `doctor` |

**Acciones permitidas:** fijar y desfijar, editar, borrar (borrado lógico, como upstream) y marcar como revisada, siempre con confirmación y por las mismas funciones del store. Sin acciones masivas destructivas.

### 8.3 TUI

Se conserva entera. Solo cambian la marca y el tema (8.4). Mismas reglas de calidad que upstream (skills `gentleman-bubbletea` y `tui-quality`).

### 8.4 Identidad visual: preparar, no decidir

El diseño (logo del arrendajo, paleta, tipografía, estilo de la TUI y la web) **lo decidirá el usuario en otra sesión con otro agente**. El agente de código no debe inventar la identidad final. Sí debe:

- Crear una única fuente de tokens de tema (`internal/qbit/theme`): colores para claro y oscuro, tipografía, espaciados. La TUI (Lip Gloss) y la web (variables CSS) leen de ella.
- Mientras tanto, usar un tema neutro provisional y un marcador de logo.
- Retirar todo el material visual de Engram (logo del elefante, imágenes de `assets/`) y no imitarlo.
- Dejar documentado en `docs/BRAND.md` dónde se enchufan logo, paleta y tipografía.

El resultado debe ser **bonito**: es un requisito explícito del usuario, y se validará con el diseño que se defina.

## 9. Mejoras por fases

Cuatro fases: la 0 garantiza paridad y la capacidad de medir; la 1 trae las mejoras con evidencia; la 2 y la 3 son hipótesis que deciden los datos. Cada mejora (M) indica su evidencia, su especificación y sus criterios de aceptación, además de los gates generales de la sección 2.

### Fase 0 — Base: paridad y medición

No es una mejora visible, pero sin ella no hay ninguna otra. Contenido detallado en la sección 11. Incluye el fork renombrado, la compatibilidad con gentle-ai, la telemetría local (sección 10) y la medición de la línea base.

### Fase 1 — Mejoras con evidencia

**M1. Espejos ODD sin truncado silencioso** (H2, G3)

- Problema: `MaxObservationLength` (50.000) corta el final del contenido. En espejos ODD eso elimina el progreso más reciente y rompe la relectura que exige ODD.
- Especificación:
  - Límite configurable más alto para documentos (propuesta: 1 MB) aplicado a los `topic_key` que coincidan con una lista de patrones configurable, por defecto `odd/*/tasks`. El resto de memorias mantiene los 50.000.
  - Por encima del límite duro: **error explícito** en lugar de truncar en silencio, para que ODD marque el espejo como pendiente según su contrato.
  - Las previews de búsqueda y contexto no cambian de tamaño.
  - Revisar los límites de sync y Cloud (`ENGRAM_CLOUD_MAX_PUSH_BYTES`, tamaño de chunks) para que un documento grande se replique bien.
- Aceptación: guardar un espejo de 120.000 caracteres y leerlo con `mem_get_observation` devuelve exactamente los mismos bytes; los tokens de contexto no cambian; sync y Cloud lo replican; test de regresión incluido.
- Candidata a proponerse a upstream (con permiso del usuario).

**M2. Historial de versiones eficiente** (H1, H3, G4)

- Problema: cada revisión guarda el contenido completo. En la base del usuario son 56 MB, el 92 % de espejos ODD.
- Especificación:
  - Comprimir el contenido de `observation_versions` (por ejemplo zstd con `klauspost/compress`, Go puro) o guardar diferencias contra la versión siguiente. Elegir con una medición sobre la base del usuario.
  - Migración aditiva (columna o marca de codificación), legible por las rutas existentes, sin cambiar el formato de sync ni las respuestas de la API.
- Aceptación: reducir el tamaño del historial al menos un 70 % en la copia de la base del usuario (meta a confirmar tras medir); leer cualquier versión devuelve el contenido idéntico; sin empeorar el p95 de `mem_save`.

**M3. Contexto por relevancia** (H4, H13)

- Problema: `FormatContext` elige solo por recencia. Acertó en el 44 % de las sesiones evaluadas; añadiendo relevancia, en el 62 %.
- Especificación:
  - Interfaz `ContextRanker` en el punto donde hoy se eligen las 20 observaciones recientes. Implementación por defecto = la de upstream.
  - Implementación de qbit-jay: candidatos = recientes + memorias que coinciden con señales del momento, ordenadas por una puntuación combinada.
  - Señales, todas locales y baratas: tokens del nombre de la rama de git (por ejemplo `feat/website-v2` ↔ `odd/website-v2/tasks`), subcarpeta de trabajo, archivos modificados en `git status`, espejos ODD activos del proyecto y un parámetro opcional nuevo `query` en `mem_context` (aditivo y compatible).
  - **Mismo presupuesto en bytes**: las memorias relevantes sustituyen a las recientes menos relevantes; nunca se añaden encima.
  - Cada memoria inyectada lleva un motivo corto ("rama", "archivo modificado", "reciente"), visible en la interfaz web; en el texto para el agente, solo si no aumenta los tokens.
  - Llamadas a git con caché y un tiempo límite (propuesta: 50 ms). Si git no está o falla, se vuelve a la recencia. El mismo ranker se aplica en todas las rutas que arman contexto: mem\_context (MCP), GET /context (HTTP, el que usan los hooks de inicio de sesión) y el contexto de compactación. En el bloque "Recent Sessions" se omiten las sesiones sin memorias ni resumen (H13); el espacio liberado se usa para memorias relevantes.
- Aceptación: en el evaluador de contexto (sección 10), al menos +10 puntos de acierto sobre la recencia; bytes inyectados ≤ línea base; p95 de `mem_context` ≤ línea base + 5 %.

### Fase 2 — Decidida con datos de telemetría

Empieza cuando haya al menos 2–4 semanas de telemetría real del usuario.

**M4. Búsqueda interlingüe y semántica** (H5; decidido por el usuario)

- Hipótesis: las consultas en español fallan contra memorias en inglés, y las búsquedas por sinónimos fallan con FTS5.
- Primero medir con telemetría: tasa de búsquedas sin resultado, reformulaciones seguidas, idioma de la consulta.
- Opciones, de menor a mayor fricción y coste, a probar en este orden:
  1. **Expansión de consulta en el protocolo:** la descripción de `mem_search` pide al agente incluir términos en inglés y sinónimos. Sin infraestructura y con un coste mínimo de tokens.
  2. **Embeddings locales** en las columnas que upstream ya reservó (`embedding`, `embedding_model`), con búsqueda por similitud por fuerza bruta en Go (miles de vectores caben de sobra) y fusión con BM25 mediante Reciprocal Rank Fusion.
- Restricciones de la opción 2: sin CGO por defecto; el modelo multilingüe no puede inflar el binario (descarga opcional bajo demanda o modelo estático pequeño tipo model2vec, a evaluar con un spike); calcular embeddings en segundo plano sin bloquear `mem_save`.
- **Antes de construir la opción 2**, revisar si upstream ya está implementándola en esas columnas y alinearse con su diseño para no chocar.
- Aceptación: en el evaluador de búsqueda con consultas reales en español e inglés, recall@5 +10 puntos o más; latencia y tamaño dentro de los gates.

**M5. Aviso de memorias posiblemente obsoletas** (H7; confirmada por el usuario, Q5)

- Idea: extraer las rutas de archivo que menciona una memoria y, al mostrarla, comprobar con git si esos archivos cambiaron mucho desde su `updated_at`. Si es así, marcarla como "posiblemente desactualizada". Nunca se oculta ni se borra.
- Aceptación: en una muestra revisada a mano, menos de un 20 % de falsos positivos; sin empeorar la latencia de búsqueda (comprobación con caché y tiempo límite).

### Fase 3 — Experimento: asimilación y aprendizaje "como un humano"

El usuario no está seguro de su utilidad. Se construye como experimento opt-in, con métricas y criterio de descarte. Upstream ya tiene piezas parecidas (`review_after`, `mem_consolidate`, relaciones), que se reutilizan.

- **M6. Refuerzo por uso:** las memorias que se recuperan y se abren a menudo ganan un pequeño empuje en el ranking; las que nunca se usan pierden peso con el tiempo (curva de olvido). Solo afecta al orden: no se borra nada. Datos: telemetría.
- **M7. Consolidación en segundo plano ("dormir"):** un proceso periódico detecta memorias episódicas repetidas (varios bugfix o decisiones sobre el mismo tema o archivos) y **propone** una memoria semántica (patrón, preferencia, regla). Si usa un LLM, va por el runner existente (`internal/llm`), es opt-in y avisa del coste. Las propuestas se revisan con `mem_review` o en la web; nunca se escriben solas en la memoria activa.
- **M8. Lecciones de errores repetidos:** si aparecen bugfix parecidos sobre los mismos archivos, sugerir una memoria de tipo `pattern` para no repetirlos.
- Criterio de descarte: tras 4 semanas, si las memorias consolidadas no se recuperan más que la media o no bajan los bugfix repetidos, se retira el experimento y se documenta el resultado.

## 10. Evaluación, benchmarks y telemetría

Ninguna mejora se acepta sin números. Esta sección define cómo se obtienen.

### 10.1 Telemetría local

- **Solo local.** Nunca sale de la máquina, no se sincroniza por Git Sync ni por Cloud y no se envía a ningún servidor.
- Activa por defecto (cero fricción) y desactivable con `QBIT_JAY_TELEMETRY=off`.
- En un archivo SQLite **separado** (`~/.qbit-jay/telemetry.db`), para no tocar el esquema principal ni la sync de upstream.
- Retención acotada (propuesta: 90 días y un tamaño máximo) y un comando para borrarla.
- Eventos:
  - **Búsqueda:** texto, proyecto, modo, número de resultados, ids devueltos (top 10), latencia, sesión.
  - **Apertura:** `mem_get_observation` de un id devuelto por una búsqueda reciente de la misma sesión (equivale a un clic).
  - **Reformulación:** otra búsqueda en la misma sesión en menos de 60 s.
  - **Contexto:** bytes y tokens estimados, ids inyectados, motivo de cada uno, y si la sesión luego los leyó o actualizó.
  - **Guardado:** tamaño, latencia, error o rechazo por límite.
- Consulta: `qbit-jay stats --telemetry` y la vista Telemetría de la web.

### 10.2 Evaluadores de calidad

| Evaluador | Qué mide | Verdad de referencia | Métricas |
| --- | --- | --- | --- |
| E1 Contexto | Si el contexto inicial trae lo que la sesión necesita | Memorias previas que la sesión revisó y, con telemetría, las que leyó | Tasa de acierto, recall dentro del presupuesto, bytes |
| E2 Búsqueda | Si la búsqueda encuentra la memoria correcta | Con telemetría: memoria abierta tras la búsqueda. Sin ella: consultas sintéticas en español e inglés generadas a partir de cada memoria | recall@5, recall@10, MRR, % sin resultados |
| E3 Rendimiento | Latencia, arranque, memoria y tamaño | Benchmarks de upstream + nuevos | p50/p95, RSS, tamaño del binario y de la base |

- E1 reproduce el método de la auditoría (sección 13): sesiones sin solapamiento en el mismo proyecto, contexto simulado en el instante de inicio de cada sesión y comparación de estrategias.
- E2: las consultas sintéticas las genera un LLM una sola vez y se guardan como fixture; el usuario puede revisar una muestra. Deben incluir consultas en español contra memorias en inglés.
- E3: reutilizar `make perf-baseline` y `make perf-check` y los benchmarks de `internal/store`. Añadir benchmarks de M1–M3 sobre corpus sintéticos de 10.000 y 100.000 memorias.

### 10.3 Datos y privacidad

- La base real del usuario **nunca** entra en el repositorio. Los evaluadores la leen desde una ruta local (`QBIT_JAY_EVAL_DB`), en solo lectura y sobre una copia.
- En el repositorio solo hay fixtures sintéticos.
- Los evaluadores se ejecutan con `go test -tags eval ./...` o un subcomando oculto de desarrollo; no forman parte del binario de release si añaden peso.

### 10.4 Línea base

Antes de la primera mejora, ejecutar E1, E2 y E3 sobre upstream sin cambios y sobre el fork renombrado, y guardar los resultados en `docs/qbit/BASELINE.md`. Todas las mejoras se comparan contra esa línea base.

## 11. Plan de trabajo

El primer entregable es la **v0.1.0: paridad total con Engram bajo el nombre qbit-jay**, con telemetría y línea base. Las mejoras empiezan después.

**Convenciones de entrega:**

- Trabajar con ODD: crear `odd/tasks/qbit-jay-fase-0.md` (y su espejo en memoria) antes de la primera escritura de código.
- Un PR por tarea, con tests primero cuando sea comportamiento nuevo y commits según la skill `commit-hygiene` de upstream.
- Cada PR incluye sus métricas frente a `docs/qbit/BASELINE.md` y la actualización de `UPSTREAM-DELTA.md` si toca código de upstream.
- Si una tarea choca con una pregunta abierta (sección 12), parar y preguntar.

&#91;embedded content: fases de qbit-jay · 4 fases con su gate\]

Cada fase empieza solo cuando la anterior supera su gate; la Fase 0 entrega la paridad completa antes de cualquier mejora.

### Fase 0 — Paridad y medición (release v0.1.0)

- [ ] **T0.1 Entorno y repositorio.** Go 1.26.11 en WSL. Importar upstream con historial completo, crear `qbitdev/qbit-jay` (owner a confirmar), remoto `upstream`. Entregable: repositorio con `main` = commit de upstream de partida y CI de tests en verde.
- [ ] **T0.2 Andamiaje.** `odd/tasks/` del proyecto, `UPSTREAM-DELTA.md`, `docs/qbit/`, `docs/BRAND.md`.
- [ ] **T0.3 Línea base de upstream.** E3 sobre upstream sin cambios.
- [ ] **T0.4 Renombrado** (sección 6.2): binario, comando, textos, carpeta de datos, variables con respaldo `ENGRAM_*`, nombre del servidor MCP en el setup, plugins y paquetes, retirada de los assets de Engram, README con atribución, tokens de tema provisionales (8.4). Los tests que comprueban textos de marca se adaptan sin perder su intención. Incluye el cambio del path del módulo Go a github.com/qbitdev/qbit-jay (Q2) y el script de sincronización de 6.1. Entregable: `qbit-jay` compila y pasa toda la suite.
- [ ] **T0.5 Migración y convivencia** (6.3): importación automática y manual desde `~/.engram`, sustitución segura de entradas de Engram en `setup`, `setup --restore`.
- [ ] **T0.6 Compatibilidad con gentle-ai** (7.3): inventario de invocaciones, test de compatibilidad y prueba real con `ENGRAM_BIN` en el WSL del usuario; comprobar el comportamiento de `eco-update`. Incluir los hooks y plugins que ejecutan engram por nombre y el paquete Pi gentle-engram que gentle-ai instala y limpia (G7). Con ese inventario, escribir docs/INTEGRATION.md, el contrato para qbit-ai (7.4).
- [ ] **T0.7 Telemetría local** (10.1) en `internal/qbit/telemetry`, con enganches mínimos.
- [ ] **T0.8 Evaluadores** E1, E2 (con fixture sintético) y E3; `docs/qbit/BASELINE.md` con upstream y con el fork.
- [ ] **T0.9 CI y releases** (6.5): GoReleaser, Homebrew tap, Scoop, npm, go install y scripts de instalación curl y PowerShell (Q7), workflows adaptados.
- [ ] **T0.10 Checklist de paridad** (8.1) completa y release **v0.1.0**.

### Fase 1 — Mejoras con evidencia (release v0.2.0)

- [ ] **T1.1 M1** Espejos ODD sin truncado silencioso.
- [ ] **T1.2 M2** Historial de versiones eficiente (medir compresión frente a diferencias y elegir).
- [ ] **T1.3 M3** Contexto por relevancia.
- [ ] **T1.4 Interfaz web** (8.2) con todas las vistas, sobre los tokens de tema provisionales. Cuando el usuario entregue el diseño, se aplica en una tarea aparte.
- [ ] **T1.5 Propuestas a upstream** de los arreglos generales (como M1), solo con permiso del usuario.

### Fase 2 — Con datos (tras 2–4 semanas de telemetría)

- [ ] **T2.1 Informe de telemetría:** búsquedas sin resultado, reformulaciones, idioma de las consultas, uso del contexto. Revisarlo con el usuario.
- [ ] **T2.2 M4** Búsqueda interlingüe: primero la expansión de consulta; embeddings solo si los datos lo justifican y tras revisar el estado de upstream.
- [ ] **T2.3 M5** Aviso de memorias obsoletas (confirmado, Q5).

### Fase 3 — Experimento de asimilación (opt-in)

- [ ] **T3.1 M6** Refuerzo por uso.
- [ ] **T3.2 M7** Consolidación en segundo plano con propuestas revisables.
- [ ] **T3.3 M8** Lecciones de errores repetidos.
- [ ] **T3.4 Revisión a las 4 semanas:** mantener o retirar según el criterio de descarte.

**Fuera de este plan:** la definición del diseño visual (otra sesión), qbit-vault, qbit-ai (solo se define su contrato de integración, 7.4) y cualquier función no listada aquí.

## 12. Riesgos y preguntas abiertas

### 12.1 Riesgos

| Riesgo | Impacto | Mitigación |
| --- | --- | --- |
| Upstream avanza muy rápido y el fork se queda atrás | Conflictos caros, se pierden mejoras de upstream | Fork delgado, sincronización semanal, `UPSTREAM-DELTA.md`, proponer arreglos generales a upstream |
| gentle-ai reinstala Engram o reescribe la configuración al actualizar | qbit-jay deja de usarse sin que nadie lo note | T0.6: usar `ENGRAM_BIN`, test de compatibilidad, revisar `eco-update`; `doctor` avisa si el agente apunta a otro binario |
| gentle-ai interpreta la salida de `version` u otros comandos | Fallos de health probe o verify | T0.6: inventario de invocaciones; si hace falta, salida compatible bajo `ENGRAM_BIN` |
| Upstream implementa embeddings de otra forma | Choque de esquema y trabajo duplicado | M4 en Fase 2 y solo tras revisar upstream |
| Renombrado incompleto | Incumplir la política de marca | Revisión con `grep -i engram` de todos los textos visibles, assets y paquetes antes de cada release |
| Muchos tests comprueban textos con la marca | Mucho trabajo en T0.4 | Adaptarlos de forma mecánica conservando su intención; nunca borrar tests |
| Pérdida de datos en la migración | Grave | Backup de SQLite, la base original nunca se modifica, test de importación |
| Las señales de git ralentizan `mem_context` | Peor latencia al arrancar | Caché, tiempo límite de 50 ms y vuelta a la recencia |
| La telemetría guarda texto de consultas | Privacidad | Solo local, archivo aparte, sin sync, retención acotada, desactivable |
| Directorio de datos en `/mnt/c` desde WSL | SQLite en modo WAL no es fiable sobre `/mnt/c` y upstream no lo garantiza como local | Documentar que los datos viven en el sistema de archivos de Linux |
| Hooks y plugins instalados por gentle-ai ejecutan "engram" por nombre | Parte del flujo seguiría usando Engram aunque gentle-ai use qbit-jay | G7 y T0.6: inventario completo; doctor detecta qué binario usa cada hook; decisión en Q11 |
| El cambio de path del módulo Go (Q2) genera conflictos en cada sincronización | Sincronizar con upstream lleva más tiempo | Script scripts/sync-upstream.sh (6.1) con la reescritura en un commit aparte; conflictos solo de imports se resuelven de forma mecánica |

### 12.2 Decisiones sobre las preguntas abiertas

El usuario respondió las 12 preguntas el 26-11-2026. Son decisiones cerradas.

| # | Pregunta | Decisión |
| --- | --- | --- |
| Q1 | Repositorio | `qbitdev/qbit-jay`, **público** |
| Q2 | Path del módulo Go | **Cambiarlo ya** a `github.com/qbitdev/qbit-jay` (ver 6.2 y 6.1 para el coste en cada sincronización) |
| Q3 | Proponer arreglos a upstream | Sí, **con aprobación del usuario caso a caso** |
| Q4 | Telemetría local | **Activa por defecto, guardando el texto de las búsquedas**; desactivable y borrable |
| Q5 | Aviso de memorias obsoletas (M5) | **Entra en la Fase 2** |
| Q6 | Cloud | Se conserva **solo autoalojada**; sin instancia de qbitdev |
| Q7 | Canales de distribución | GitHub Releases + Homebrew tap, Scoop, npm (plugin de Pi), `go install`, script `curl` (Linux, macOS, WSL) y script de PowerShell para Windows (ver 6.5) |
| Q8 | Integración con gentle-ai | Compatibilidad con gentle-ai vía `ENGRAM_BIN` **y** qbit-jay como componente nativo del nuevo ecosistema **qbit-ai** (ver 7.4) |
| Q9 | Versiones | Numeración propia desde **0.1.0**; `qbit-jay version` indica la versión de Engram de la que parte, siempre que gentle-ai no dependa del formato |
| Q10 | Prompts con secretos | **Revisarlo cuando exista qbit-vault** |
| Q11 | Hooks y plugins que llaman a `engram` | **qbit-jay trae sus propios plugins y qbit-ai los instala**; con gentle-ai, `qbit-jay setup` sustituye los de Engram |
| Q12 | Corte de prompts a 2.000 caracteres | **Se mantiene** |

Si durante el desarrollo surge una duda nueva que cambia el comportamiento visible, el agente pregunta antes de decidir.

## 13. Apéndices

### 13.1 Método de la auditoría

- **Origen:** copia de `~/.engram/engram.db` (con WAL y SHM) hecha desde WSL a `Documents\qbit-audit` el 26-11-2026, abierta en solo lectura. `PRAGMA integrity_check` = `ok`.
- **Perfil (H1, H3, H8–H10):** recuentos SQL sobre `observations` (vivas = `deleted_at IS NULL`), `observation_versions`, `sessions`, `user_prompts` y `memory_relations`.
- **Espejos ODD (H1–H3):** `topic_key LIKE 'odd/%'`; truncados = contenido que termina en `... [truncated]`; historial = suma de `length(content)` en `observation_versions` unida a sus observaciones.
- **Casi duplicados (4.1):** Jaccard ≥ 0,5 sobre trigramas de palabras de título + contenido, dentro de cada proyecto, excluyendo `session_summary` y pares con tamaños muy distintos.
- **Contexto (H4):** sesiones con fin registrado, con prompts y sin otra sesión solapada en el mismo proyecto. "Necesaria" = memoria creada antes del inicio de la sesión y con una versión creada durante ella. Recencia = las 20 memorias no fijadas más recientes del proyecto creadas antes del inicio. "Mencionada" = algún segmento del `topic_key` necesario (sin `odd`, `tasks`, `history`) aparece en el primer prompt. Resultado: 39 sesiones; recencia 17 (44 %); mención 17; mención o recencia 24 (62 %).
- **Idioma (H5):** muestras aleatorias de 400 prompts y 400 memorias; "español" si contienen palabras funcionales como *que, el, la, los, para, con, una, por*.
- **Rutas (H7):** expresión regular de rutas con extensiones de código y documentación habituales, sobre memorias que no son resúmenes.
- **Secretos (H11):** patrones de formatos conocidos (GitHub, Anthropic, OpenAI, AWS, Google, Slack, claves privadas, JWT, URL con contraseña) más un patrón genérico de asignación, clasificado por entropía sin mostrar valores. El patrón genérico dio unos 2.100 falsos positivos (casi todos en el historial de versiones).
- **Limitaciones:** ver 4.3. Los scripts se ejecutaron fuera del repositorio; el agente debe reimplementarlos como evaluadores (10.2), no copiarlos.
- **Copia de la base:** sigue en `C:\Users\LeandroA\Documents\qbit-audit`. Contiene datos privados: el usuario puede borrarla cuando quiera o reutilizarla como entrada de `QBIT_JAY_EVAL_DB`.
- **Sesiones vacías (H10, H13):** sesiones sin observaciones y sin prompts. Para H13 se reprodujo la consulta de `recentSessions` (con `LEFT JOIN`) en 200 instantes de inicio de sesión aleatorios de `web-ai-mate`: 680 de 996 entradas tenían 0 memorias.
- **Prompts truncados (H12):** distribución de longitudes de `user_prompts` (223 de exactamente 2.003 caracteres) y lectura del código: `truncate(stripPrivateTags(...), 2000)` en `plugin/pi/index.ts` y `plugin/opencode/engram.ts`.

### 13.2 Constantes de upstream que importan

| Constante | Valor | Dónde |
| --- | --- | --- |
| `MaxObservationLength` | 50.000 caracteres | `internal/store/store.go` (`DefaultConfig`) |
| `MaxContextResults` | 20 | `internal/store/store.go` |
| `memContextDefaultMaxBytes` | 16.384 | `internal/mcp/mcp.go` |
| `memContextMaxBytesCeiling` | 65.536 | `internal/mcp/mcp.go` |
| Presupuesto de historial antiguo | 2.048 bytes o `MaxBytes/4` | `FormatContextWithOptionsContext` |
| Pesos BM25 | título 5, contenido 1, `topic_key` 3 | `searchFTS*Weight` |
| Boosts | fijada 0,10; recencia 0,06 (30 días); estabilidad 0,04 | `search*Boost` |
| Tokenizador FTS5 | `trigram remove_diacritics 1` | `observations_fts` |
| Revisión por tipo | 3, 6 o 12 meses | `decayReviewAfterMonths` |

### 13.3 Referencias

- [Engram (upstream)](https://github.com/Gentleman-Programming/engram): `README.md`, `DOCS.md`, `TRADEMARKS.md`, `CONTRIBUTING.md`, `AGENTS.md`, `docs/COMPARISON.md`, `docs/ARCHITECTURE.md`, `docs/AGENT-BEHAVIOR-EVIDENCE.md`, `docs/INSTALLATION.md`.
- [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai): `internal/components/engram/` (`caller_binary.go`, `inject.go`, `download.go`, `healthprobe.go`, `verify.go`) e `internal/assets/skills/_shared/` (`engram-convention.md`, `persistence-contract.md`).
- Clayton, N. S. y Dickinson, A. (1998). "Episodic-like memory during cache recovery by scrub jays". *Nature* 395, 272–274. Origen del nombre.
