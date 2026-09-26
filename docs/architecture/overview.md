# Visión General de Arquitectura

`gz-ia` está estructurado siguiendo una arquitectura desacoplada y modular en capas concéntricas, donde la responsabilidad de cada paquete está rígidamente delimitada para garantizar extensibilidad, testeabilidad y resistencia a fallos.

---

## Diagrama de Capas del Sistema

```mermaid
flowchart TB
    subgraph UI ["1. Capa de Clientes y Entrada"]
        CLI["CLI Commands (spf13/cobra) - internal/clients/cli"]
        TUI["TUI Fastfetch y Forms - internal/clients/tui"]
    end

    subgraph CoreFeatures ["2. Capa de Servicios de Dominio (Features)"]
        SESS["Session Service y Drivers - internal/features/session"]
        WORK["Workspace y Git Worktree Manager - internal/features/workspace"]
        LOG["Event Logger y Bus - internal/features/logger"]
        METR["Metrics y Telemetry Aggregator - internal/features/metrics"]
        VAULT["Secret Vault y Envs - internal/features/vault"]
        TOOL["Modular Tooling y Perfiles - internal/features/tooling"]
        UPDT["Release y GitHub Updater - internal/features/updater"]
    end

    subgraph KernelLayer ["3. Microkernel Orchy (Motor MCP y Plugins)"]
        KERNEL["Microkernel Core y IoC Container - packages/orchy/core"]
        BUS["Typed Event Bus y RPC - packages/orchy/events"]
        MCP["MCP Stdio Server (JSON-RPC 2.0) - packages/orchy/mcp"]
        TOOLS["Tool Registry y Circuit Breaker - packages/orchy/tools"]
        BATT["Worktree Batteries (Read) - packages/orchy/batteries/worktree"]
    end

    subgraph Storage ["4. Persistencia Local y Git"]
        SESS_JSON[".harness/sessions/{id}.json"]
        SESS_MANIFEST[".harness/sessions/{id}.manifest.json"]
        EVENTS_JSONL[".harness/sessions/{id}.events.jsonl"]
        VAULT_JSON[".harness/vault.json (Permisos 0600)"]
        WORKTREE_DIR[".harness/worktrees/{id} (branch harness/{id})"]
    end

    CLI --> SESS
    CLI --> WORK
    CLI --> LOG
    CLI --> METR
    CLI --> VAULT
    CLI --> TOOL
    CLI --> UPDT
    TUI --> CLI

    SESS --> WORK
    SESS --> LOG
    SESS --> METR
    SESS --> VAULT
    SESS --> SESS_JSON
    SESS --> SESS_MANIFEST
    LOG --> EVENTS_JSONL
    VAULT --> VAULT_JSON
    WORK --> WORKTREE_DIR

    SESS -.-> KERNEL
    KERNEL --> BUS
    KERNEL --> TOOLS
    KERNEL --> MCP
    MCP --> TOOLS
    BATT --> TOOLS
```

---

## Detalle de las Tres Capas Principales

### 1. Capa de Clientes (`internal/clients/`)

Esta capa maneja la interacción directa con el usuario, procesa argumentos y formatea las respuestas:

- **`internal/clients/cli`:** Construida sobre `spf13/cobra`. Proporciona el árbol completo de comandos y subcomandos (`chat`, `session`, `vault`, `mcp`, `update`, `version`). No contiene lógica de negocio; valida parámetros e invoca a los servicios de dominio.
- **`internal/clients/tui`:** Construida con `charmbracelet/bubbletea`, `lipgloss` y `huh`. Proporciona la experiencia visual guiada con menús accesibles, banners de telemetría y diálogos interactivos de confirmación.

### 2. Capa de Servicios de Dominio (`internal/features/`)

El núcleo operativo de la CLI de `gz-ia`, completamente desacoplado de la terminal:

- **`session` (`internal/features/session`):** Orquesta el ciclo de vida de cada sesión, gestiona los drivers de los agentes (`agy`, `claude`, `opencode`, `pi-agent`), controla procesos en segundo plano y almacena atómicamente la metadata en `.harness/sessions/<id>.json`.
- **`workspace` (`internal/features/workspace`):** Administra el ciclo de vida de los **Git Worktrees** efímeros, la creación de ramas `harness/<id>`, el manifiesto de proyección atómico (`.manifest.json`) y el fallback seguro en directorios planos si Git no está inicializado.
- **`vault` (`internal/features/vault`):** Almacén seguro de secretos y variables de entorno centralizadas (`.harness/vault.json` con permisos `0600`), enmascaramiento de valores y detección automática de credenciales faltantes.
- **`tooling` (`internal/features/tooling`):** Gestión y composición de perfiles agénticos y toolkits modulares (`tooling/config.json`, directivas `AGENTS.md`, `rules/`, `skills/` y registro dinámico de herramientas en el microkernel).
- **`logger` (`internal/features/logger`):** Captura y persiste el log estructurado de etapas de razonamiento y acciones en formato JSONL (`.events.jsonl`).
- **`metrics` (`internal/features/metrics`):** Agrega y analiza métricas de ejecución: duración total, pasos completados, consumo de tokens (input, output, caché) y llamadas a herramientas por cada agente y subagente.
- **`updater` (`internal/features/updater`):** Comprueba actualizaciones y nuevas versiones publicadas en GitHub Releases, gestiona descargas y reemplaza atómicamente el ejecutable.

### 3. Microkernel Orchy (`packages/orchy/`)

Un paquete autónomo y reutilizable ubicado en `packages/orchy` que provee:

- **Inversión de Control (IoC):** `ServiceContainer` y `KernelContext` para registrar dependencias desacopladas.
- **Honest Microkernel & Circuit Breaker:** Supervisa el estado de salud de cada herramienta registrada (`HEALTHY`, `DEGRADED`, `DEAD`) mediante `ToolProxy`.
- **Servidor MCP Nativo:** Exposición de herramientas a cualquier agente compatible con el protocolo MCP (Model Context Protocol) a través de canales estándar (`io.Reader` / `io.Writer`).
- **Baterías Incluidas:** Plugins de worktrees para inspección segura (`worktree_read` expuesta a agentes por defecto) e integración controlada (`worktree_get`).

---

## Flujo de Ejecución de una Sesión de Chat

```mermaid
sequenceDiagram
    autonumber
    actor User as "Usuario / Terminal"
    participant CLI as "gz-ia CLI"
    participant Driver as "Multi-Driver (agy/claude)"
    participant Work as "Workspace Provider"
    participant Git as "Git Engine"
    participant Store as "Session Store (.harness/)"

    User->>CLI: gz-ia chat -p agy -m supervised
    CLI->>Driver: Resolver y validar binario (LookPath)
    Driver-->>CLI: Driver disponible (OK)
    CLI->>Work: Prepare sessionID
    Work->>Git: git worktree add .harness/worktrees/{id} -b harness/{id}
    Git-->>Work: Worktree inicializado
    CLI->>Store: Create SessionRecord RUNNING
    Store-->>CLI: Guardado en .harness/sessions/{id}.json
    CLI->>Driver: Launch(WorktreePath, SupervisedFlags)
    Note over Driver,User: Sesión interactiva TTY conectada
    Driver->>User: Preguntas y ejecuciones de comandos
    User-->>Driver: Aprobación e interacción
    Driver-->>CLI: Finalización del proceso (Exit code 0)
    CLI->>Store: Complete SessionRecord COMPLETED
    CLI->>User: Sesion completada con exito
```
