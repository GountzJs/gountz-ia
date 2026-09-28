# Historial de Versiones (Changelog)

Todos los cambios notables en este proyecto serán documentados en este archivo.

El formato está basado en [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/) y este proyecto se adhiere a [Semantic Versioning](https://semver.org/lang/es/).

---

## [0.4.0] - 2026-09-28

### Memoria Semántica, Contexto Centralizado y Soberanía Estricta de Git

Esta versión introduce la infraestructura de memoria de contexto y decisiones (`internal/features/memory`), la consolidación de eventos como fuente centralizada de estado de sesión (`SessionContext`) y la soberanía estricta de Git en la integración de cambios.

#### Novedades y Mejoras

- **Memoria de Contexto y Decisiones (`internal/features/memory`):**
  - Almacenamiento dual semántico: memoria local por sesión (`.harness/sessions/<id>.memory.json`) y global del proyecto (`.harness/memory.json`).
  - Motor de búsqueda semántica con ranking Okapi BM25 (`BM25Search`) tokenizado con ponderación por título, etiquetas y contenido.
  - Batería de herramientas MCP nativas (`memory_save`, `memory_search`, `memory_list`, `memory_consolidate`).
  - Subcomandos CLI `gz-ia memory` (`save`, `search`, `list`, `consolidate`).
- **Events como Fuente Centralizada de Estado (`internal/features/session`):**
  - Metadata enriquecida en eventos de observabilidad `START` y `FINISH`.
  - Estructura `SessionContext` para handoff continuo entre agentes con el historial completo de eventos y diff de archivos.
  - Subcomando CLI `gz-ia session context <id>` con soporte de salida formateada o JSON (`-j, --json`).
- **Soberanía Estricta de Git en `gz-ia session get`:**
  - Rediseño de `session.Service.Get`: trae las modificaciones y archivos creados desde el worktree de sesión directamente al directorio de trabajo activo como cambios no preparados (*unstaged*).
  - Eliminación de banderas obsoletas `--squash` y `--no-commit`, evitando git merge commits intermediarios o conflictos en el historial de Git.

---

## [0.3.0] - 2026-09-28

### Microkernel Orchy Modular, Observabilidad en Vivo y Auto-Updater

Esta versión consolida el paquete modular `packages/orchy`, el soporte de observabilidad en vivo para eventos de sesión y el sistema de actualización atómica desde Forgejo y Sonatype Nexus.

#### Novedades y Mejoras

- **Reorganización del Microkernel Orchy (`packages/orchy/`):**
  - Decoupling del microkernel en subsistemas independientes (`core`, `events`, `tools`, `plugins`, `batteries`, `mcp`).
  - Honest Kernel con Circuit Breaker de 3 estados (`HEALTHY`, `DEGRADED`, `DEAD`) sobre proxies de herramientas (`ToolProxy`).
  - Bus desacoplado de eventos Pub/Sub y RPC síncrono con control de context (`events.EventBus`).
- **Observabilidad en Tiempo Real (`internal/features/logger`):**
  - Streaming en vivo de eventos `.events.jsonl` mediante `gz-ia session logs <id> -f`.
  - Batería de observabilidad MCP `ObservabilityPlugin` con la herramienta `session_log`.
- **Actualizaciones Atómicas (`internal/features/updater`):**
  - Verificación e instalación de releases binarias (`gz-ia update`) contra Forgejo y Sonatype Nexus, previniendo fallos `ETXTBSY`.
- **Detección Untracked Recursiva (`internal/features/workspace`):**
  - Cálculo de diffs incluyendo archivos no seguidos en subdirectorios profundos (`git status --porcelain`).

---

## [0.2.0] - 2026-09-28

### Observabilidad MCP, Prompt Orquestador y Cobertura de Tests

Esta versión introduce el plugin nativo de observabilidad para agentes vía MCP, el prompt de orquestador por defecto en el CLI, el rastreo de archivos proyectados en el manifiesto de sesión, cobertura ampliada de tests en `metrics`, `logger`, `session` y `cli`, y la incorporación de los tres toolkits de equipo al repositorio.

#### Novedades y Mejoras

- **Plugin `ObservabilityPlugin` (`packages/orchy/batteries/observability/`):**
  - Herramienta MCP `session_log` que permite a agentes emitir eventos de trazabilidad (etapas `READ`, `PENDING`, `FINISH`) al log de sesión activa desde cualquier contexto MCP, sin acceso directo al sistema de archivos.
- **Prompt de Orquestador por Defecto (`internal/features/session/service.go`):**
  - Método `DefaultOrchestratorPrompt` que genera el prompt inicial cuando `gz-ia chat` se invoca sin la bandera `-i`, consolidando el contexto de sesión, workspace y toolkits activos.
- **Campo `projected_files` en Sesión (`internal/features/session/session.go`, `store.go`):**
  - Rastreo persistente de archivos proyectados por toolkits en el worktree dentro del manifiesto de sesión, habilitando fusiones no destructivas y auditoría de proyección.
- **Manifiesto de Workspace Enriquecido (`internal/features/workspace/manifest.go`):**
  - Campos `toolkits` y `projected_files` añadidos al manifiesto de workspace con serialización completa y tests de cobertura (`manifest_test.go`).
- **`ProjectIntoWorktree` Mejorado (`internal/features/tooling/service.go`):**
  - Escritura del manifiesto enriquecido de archivos proyectados (`ProjectedFiles`) durante la proyección de toolkits en el worktree.
- **Cobertura de Tests Ampliada:**
  - `internal/features/session/session_test.go` y `service_test.go`: ciclo de vida completo de sesión, manifiesto, prune y cleanup.
  - `internal/features/metrics/service.go` y `metrics_test.go`: cobertura de `EstimateTokens` y sesiones sin transcripción.
  - `internal/features/logger/store.go` y `logger_test.go`: filtrado por `SessionID` en lectura de JSONL y cobertura ampliada de watch/emit.
  - `internal/clients/cli/`: tests de integración para `chat`, `mcp`, `session` y detección de `DefaultOrchestratorPrompt`.
- **Toolkits de Equipo en Repositorio (`.agents/toolkits/`):**
  - Tres toolkits versionados añadidos al repositorio: `TOOLKIT_GZ_IA-AGENTS.md`, `TOOLKIT_MAINTAINER_GZ_IA-AGENTS.md` y `TOOLKIT_CONVENTIONAL_COMMIT-AGENTS.md`.

---

## [0.1.0] - 2026-09-27

### Consolidación — Toolkits de Equipo en Git, Seguridad en Vault y Estabilidad de Terminal

Esta versión marca la transición a **v0.1.0**, incorporando toolkits compartidos para equipos en Git, saneamiento del ciclo de vida de procesos interactivos, respeto irrestricto a suscripciones oficiales de Claude Code y compatibilidad documentada para entornos frontend modernos.

#### Novedades y Mejoras

- **Toolkits de Equipo Versionados en Git (`.gz-ia/toolkits/` y `toolkits/`):**
  - Soporte en `internal/features/tooling/loader.go` para descubrir y crear toolkits dentro de `.gz-ia/toolkits/<id>` y `toolkits/<id>`.
  - Permite que los equipos compartan directivas, reglas, skills procedimentales y herramientas MCP directamente en el repositorio Git del proyecto (sin ser excluidos por `.harness/`).
- **Respeto a Suscripciones Oficiales (Claude Code y OpenCode):**
  - Eliminación de la validación hardcodeada de `ANTHROPIC_API_KEY` y `OPENAI_API_KEY` en `session.Service`.
  - Claude Code opera de forma nativa con suscripción Pro/Team vía OAuth (`claude login`) sin empujar al usuario a gastar saldo de API por tokens.
- **Estabilidad de Terminal TTY y Señal `SIGTTOU`:**
  - Protección explícita con `signal.Ignore(syscall.SIGTTOU, syscall.SIGTTIN)` en `sysproc_unix.go` durante la restauración del control de la terminal interactiva, evitando que shells como Bash o Zsh suspendan el proceso `gz-ia`.
- **Saneamiento de Dependencias y Dev Servers Frontend:**
  - Corrección de la ruta del symlink hacia la raíz del proyecto a 3 niveles (`../../../node_modules`).
  - Documentación de exclusiones críticas para evitar recargas continuas en dev servers (Vite `server.watch.ignored: ['**/.harness/**']`) y escaneos de Tailwind CSS.
  - Advertencia sobre el comportamiento de Git ante reglas con trailing slash (`node_modules/` no ignora enlaces simbólicos de archivos).
- **Desinstalación Idempotente y Limpieza de Configuración:**
  - Comando de poda de ramas robusto mediante `git for-each-ref` y `xargs -r git branch -D`.
  - Alerta previa sobre el borrado irreversible de secretos en `.harness/vault.json`.
  - Instrucciones para limpiar la entrada `.harness` en `.git/info/exclude` y la directiva `extensions.worktreeConfig`.
- **Estandarización de Estilo e Interfaz:**
  - Redacción completamente unificada en tuteo neutro en toda la documentación.
  - Depuración de emojis saturados en títulos y mensajes de CLI/TUI, consolidando glifos tipográficos sobrios y consistentes.

---

## [0.0.3] - 2026-09-27

### Tercera Entrega — Toolkits Modulares, Presets Emergentes y Refactor de Tooling

Esta versión consolida el modelo composable donde el **Toolkit** es la unidad atómica de capacidad y el **Perfil / Preset** es una composición emergente, eliminando el antiguo paquete monolítico `profile` y potenciando la experiencia interactiva tanto en CLI como en TUI.

#### ✨ Novedades y Mejoras

- **Transición a Toolkits Modulares y Presets Emergentes (`tooling`):**
  - Reemplazo completo del modelo estático monolítico de `profile` por la arquitectura desacoplada de `tooling`: el **Toolkit** como unidad atómica y el **Preset** como composición emergente.
  - Eliminación definitiva del paquete `internal/features/profile/` (`composer.go`, `model.go`, `profile_test.go`, `projector.go`, `service.go`, `store.go`) y el archivo CLI `internal/clients/cli/profile.go`.
  - Nuevo comando CLI `gz-ia toolkit` con subcomandos `list`, `show`, `create` (alias `init`), `skills` y `path`, manteniendo `profile`, `profiles` y `toolkits` como aliases de retrocompatibilidad.
  - Scaffolding estándar para toolkits con `toolkit.json`, directivas maestras (`AGENTS.md`), reglas (`rules/example.md`), habilidades (`skills/example/SKILL.md`) y herramientas ejecutables (`tools.json`).
  - Síntesis dinámica en caliente de `AGENTS.md` y proyección no destructiva de enlaces simbólicos para habilidades y reglas en worktrees.
- **Evolución del CLI (`gz-ia chat` y `gz-ia mcp`):**
  - Nueva bandera `-T, --toolkit <id>` en `gz-ia chat` para activar toolkits modulares específicos (repetible o separado por comas).
  - Bandera `-P` ahora representa `--preset`, manteniendo `--profile` como alias de compatibilidad.
  - Registro dinámico en el microkernel Orchy de herramientas declaradas en `tools.json` o scripts en `tools/` protegidas por Circuit Breaker.
- **Mejoras Integrales en la TUI (`internal/clients/tui`):**
  - Selector interactivo de toolkits y presets en el menú de chat.
  - Exploración y configuración interactiva del Vault de secretos.
  - Visualización y manejo reactivo mejorado para terminales con Fastfetch.

---

## [0.0.2] - 2026-09-26

### Segunda Entrega — Vault de Secretos, Manifiesto de Worktrees y Tooling Modular

Esta versión introduce el almacén seguro de secretos (`vault`), la infraestructura de proyección agéntica basada en manifiesto para fusiones no destructivas, soporte interactivo avanzado de terminales TTY y grupos de procesos, y el servidor MCP desacoplado con toolkits modulares.

#### ✨ Novedades y Mejoras

- **Manifiesto de Proyección y Fusión No Destructiva (`workspace`):**
  - Nuevo manifiesto de sesión persistido en `.harness/sessions/<id>.manifest.json` que rastrea `CreatedFiles`, `OriginalFiles` (contenido previo a la proyección) y `ProjectedHash` (SHA-256).
  - Eliminación definitiva de la dependencia frágil de `core.excludesFile`.
  - Operaciones `session get` y `session diff` 100% no destructivas: nunca borran archivos legítimos del repositorio base (como `.agents/config.json` o `.mcp.json` del equipo) y preservan intactas las ediciones intencionales realizadas por el agente (p.ej. modificaciones a `AGENTS.md`).
  - Purga limpia y atómica de artefactos efímeros proyectados antes del commit de merge.
- **Vault Centralizado de Secretos y Variables de Entorno (`gz-ia vault`):**
  - Almacén local protegido en `.harness/vault.json` con permisos estrictos POSIX `0600` e ignorado automáticamente por Git.
  - Subcomandos `list`, `set`, `get`, `delete` (con aliases `rm`, `remove`) y `path`.
  - Formulario interactivo enmascarado (`huh.EchoModePassword`) al omitir el valor en `set`, evitando fugas en el historial de shell (`.bash_history`).
  - Ofuscación visual de credenciales (`MaskSecret`) en `list` y `get` con opción de revelado explícito (`--reveal`).
  - Detección automática no bloqueante de variables faltantes al iniciar sesiones interactivas (`gz-ia chat` o TUI) según los proveedores y perfiles activos.
  - Inyección transparente y segura de secretos en el proceso hijo (`OSRunner`) sin contaminar el entorno global del usuario.
- **Toolkits Modulares y Servidor MCP Desacoplado (`gz-ia mcp`):**
  - Arquitectura de tooling modular en `~/.config/gz-ia/tooling/config.json` que compone múltiples toolkits reutilizables con directivas `AGENTS.md`, `rules/`, `skills/` y herramientas ejecutables `tools.json`.
  - Servidor MCP nativo desacoplado ejecutable mediante `gz-ia mcp --session <id> --tooling <dir>`.
  - Detección automática del worktree de la sesión y activación dinámica de perfiles y toolkits en el microkernel Orchy.
- **Control de Terminal TTY y Grupos de Procesos en Killer:**
  - Configuración avanzada de procesos en sistemas POSIX (`configureSysProcAttr`): asignación de grupo de procesos (`Setpgid: true`) y elevación a Foreground de la terminal (`Ctty: fd`) para evitar señales `SIGTTIN` en modo interactivo y capturar limpiamente `Ctrl+C`.
  - Terminación ordenada por grupo de procesos (`killProcessGroup`): despacho de `SIGTERM` al grupo completo (`target = -pgid`), periodo de gracia progresivo de hasta 1.5 segundos (1500ms) y escalado automático a `SIGKILL` si algún subproceso o servidor MCP continúa con vida. En Windows, podado recursivo con `taskkill /T /F /PID`.
- **Reenvío Seguro de Hooks y Conventional Commits:**
  - Reenvío transparente de hooks de validación del proyecto (`pre-commit` y `commit-msg`) desde el worktree hacia el repositorio base, asegurando compatibilidad con Husky, lint-staged y commitlint.
  - Estandarización de commits internos de seguridad y merges automáticos bajo la especificación **Conventional Commits**: `chore(harness): session <id> changes` y `chore(harness): merge session <id> changes`.
- **Infraestructura de Despliegue y Releases:**
  - Soporte de tags flexibles (`0.0.2` y `v0.0.2`) en el script universal de instalación (`install.sh`) y flujos de GitHub Actions.
  - Actualizador de releases (`gz-ia update`) adaptado a las convenciones de etiquetado de GitHub Releases.

---

## [0.0.1] - 2026-09-24

### Lanzamiento Inicial — Fundación del Harness

Esta versión marca la primera entrega oficial de `gz-ia`, estableciendo las bases de la infraestructura universal de terminal para agentes de inteligencia artificial.

#### ✨ Características Principales

- **Arquitectura Agnóstica Multi-Driver:**
  - Soporte de primer orden para 4 agentes de terminal: Google Antigravity (`agy`), Anthropic Claude Code (`claude`), OpenCode (`opencode`) y Pi Agent (`pi-agent`).
  - Normalización de permisos en tres niveles: `readonly`, `supervised` y `autonomous`.
  - Detección automática en vivo de binarios con comandos sugeridos de instalación.
- **Aislamiento en Git Worktrees Efímeros:**
  - Creación automática de árboles de trabajo en `.harness/worktrees/<id>` vinculados a ramas dedicadas `harness/<id>`.
  - Inspección limpia de diferencias de código con `session diff` y `session read`.
  - Integración atómica hacia la rama principal con `session get` (`--squash`, `--no-commit`).
  - Fallback transparente sin interrupción en directorios donde Git no esté inicializado.
- **Microkernel Orchy & Servidor MCP:**
  - Contenedor IoC de servicios (`ServiceContainer`) y bus tipado de eventos (`EventBus`).
  - Patrón Circuit Breaker de 3 estados (`HEALTHY`, `DEGRADED`, `DEAD`) sobre proxies de herramientas (`ToolProxy`).
  - Servidor MCP nativo sobre Stdio con especificación JSON-RPC 2.0 y generación automática de manifiestos.
  - Baterías de worktrees incluidas (`worktree_read` por defecto para inspección del agente, `worktree_get` para integración controlada).
- **TUI Interactiva con Fastfetch & CLI-First:**
  - Interfaz visual accesible construida con Bubble Tea, Lipgloss y formularios Huh.
  - Paridad funcional del 100% entre la TUI y la línea de comandos.
- **Observabilidad y Telemetría:**
  - Logger estructurado de etapas (`READ` $\rightarrow$ `PENDING` $\rightarrow$ `FINISH`) persistido en formato `.events.jsonl`.
  - Agregador de métricas con cálculo de duración, recuento de pasos y desglose de tokens y llamadas a herramientas.
- **Actualizador Atómico de GitHub Releases:**
  - Verificación continua e instalación atómica de releases binarias desde GitHub Releases.
