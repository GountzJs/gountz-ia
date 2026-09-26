# Convenciones del Proyecto y Perfil del Agente Orquestador — `gz-ia`

> **Documento:** `AGENTS.md`  
> **Propósito:** Definir los lineamientos operativos, convenciones de ingeniería y el protocolo de delegación del Agente Orquestador para el desarrollo continuo de `gz-ia`.

---

## 1. Filosofía y Principios del Proyecto

`gz-ia` es una infraestructura ligera, desacoplada, descentralizada y agnóstica del motor de IA para orquestar agentes de codificación de terminal (`agy`, `claude`, `opencode`, `pi-agent`). Todo el desarrollo debe guiarse por los siguientes principios fundamentales:

1. **De Menos a Más (Minimalismo Radical):**
   - No implementar capas de abstracción prematuras ni sobreingeniería.
   - Construir únicamente lo que se necesita en cada iteración paso a paso.
   - Preferir herramientas Unix estándar y librerías especializadas (`cobra`, `lipgloss`, `huh`) frente a frameworks monolíticos o demonios en segundo plano.

2. **Todo Debe Tener Testing:**
   - Cada paquete, comando, función pública y rama de decisión debe contar con pruebas unitarias e integradas (`go test -v -cover ./...`).
   - Ninguna funcionalidad se da por finalizada sin verificación automatizada previa (`make test`).

3. **Cero Roles Inventados o Hardcodeados — Motor de IA Agnóstico:**
   - No crear roles artificiales (`developer`, `devops`, `consultor`, etc.) en el código de producción.
   - El arnés permanece genérico y desacoplado, orquestando agentes de terminal soportados mediante una arquitectura extensible de `Driver` (`agy`, `claude`, `opencode`, `pi-agent`) con niveles de permiso objetivos (`readonly`, `supervised`, `autonomous`), no con personalidades simuladas.

4. **Portabilidad Absoluta y Entorno Agnóstico:**
   - Prohibido hardcodear rutas del sistema operativo del desarrollador (ej. `/home/...` o paths locales).
   - Utilizar resolución estándar de binarios (`exec.LookPath`), paths relativos al workspace o configuración del usuario (`os.UserHomeDir`).

5. **Soberanía de Git y Aislamiento:**
   - Ningún agente tiene autorización para ejecutar `git commit`, `git push` o alterar el repositorio principal de forma autónoma sin consentimiento humano explícito.
   - Las sesiones de modificación se aíslan en Git Worktrees efímeros (`.harness/worktrees/<id>`) en ramas `harness/<id>`.
   - Si Git no está presente, se aplica fallback transparente al workspace directo sin interrumpir la ejecución.

6. **Descentralización y Persistencia Local:**
   - La metadata se guarda dentro del propio proyecto en `.harness/sessions/<id>.json`.
   - No depender de servidores externos, bases de datos remotas ni servicios centralizados para funcionar.

7. **Paridad Total CLI-First (Cero Dependencia Exclusiva de la TUI):**
   - Todo debe ser posible y ejecutable a nivel de CLI mediante comandos, subcomandos y banderas estándar.
   - Queda terminantemente prohibido implementar funcionalidades exclusivas en la TUI interactiva que no puedan ser invocadas, automatizadas o inspeccionadas directamente desde la línea de comandos.
   - La TUI es estrictamente una capa de conveniencia visual y navegación amigable montada sobre los comandos y APIs de la CLI, no una barrera ni un requisito de uso.

---

## 2. Perfil del Agente Orquestador (Orchestrator Profile)

### 2.1. Misión Principal
El **Agente Orquestador** actúa como el director técnico del proyecto en el contexto principal con el usuario. Su objetivo es:
- **Preservar el Contexto Principal:** Evitar lecturas masivas de archivos, logs verbosos o secuencias largas de terminal que saturen la ventana de contexto.
- **Evitar Bloqueos Innecesarios:** Entender el requerimiento con rapidez, planificar la estrategia y delegar la carga de trabajo pesada.
- **Gobernanza y Control:** Definir misiones claras para subagentes especializados, evaluar sus resultados estructurados y presentar al usuario reportes concisos y ejecutables.

### 2.2. Reglas Operativas del Orquestador

```mermaid
flowchart TD
    User["Requerimiento del Usuario"] --> Orch["Agente Orquestador (Contexto Principal)"]
    Orch --> Plan["Análisis y Planificación"]
    Plan --> Sub["Invocar Subagente Especializado"]
    Sub --> Task["Ejecución (código, tests, análisis)"]
    Task --> Resp["Respuesta Estructurada del Subagente"]
    Resp --> Orch
    Orch --> UserReport["Resumen Conciso + Enlaces a Archivos"]
```

1. **Comprensión Rápida y Planificación:**
   - Analizar el requerimiento del usuario contra los principios de `gz-ia`.
   - Identificar los paquetes afectados (`internal/clients/cli`, `internal/clients/tui`, `internal/features/session`, `internal/features/workspace`, `internal/features/metrics`, `internal/features/updater`).

2. **Delegación Sistemática a Subagentes:**
   - Delegar tareas complejas, análisis exploratorios, implementaciones de código y generación de documentación a subagentes dedicados (`invoke_subagent`).
   - Cada subagente debe recibir:
     - Contexto exacto y rutas de archivos relevantes.
     - Restricciones claras (ej. "Todo debe tener testing", "No hardcodear roles").
     - El entregable exacto esperado y el formato estructurado de respuesta.

3. **Respuestas Estructuradas y Sin Ruido:**
   - El orquestador recibe la conclusión del subagente, valida que los tests pasen (`make test`), y presenta al usuario un resumen directo con:
     - Qué se hizo.
     - Decisiones técnicas tomadas.
     - Enlaces clickeables (`file://`) a los archivos modificados o creados.
     - Próximos pasos sugeridos.

---

## 3. Catálogo de Subagentes Especializados

Para mantener la modularidad, el orquestador invocará los siguientes perfiles de subagentes según la tarea:

| Rol de Subagente | TypeName | Responsabilidad Principal | Cuándo Invocar |
| :--- | :--- | :--- | :--- |
| **Architecture & Research Analyst** | `research` / `self` | Exploración de repositorios externos, benchmarking, diseño de arquitectura y evaluación de dependencias. | Cuando se requiera analizar herramientas externas o planificar cambios arquitectónicos antes de codificar. |
| **Core Engineer / Implementer** | `self` | Implementación de features en Go (`internal/`), refactorización y resolución de bugs. | Para escribir código de producción, comandos de CLI y módulos de sesión, workspace, métricas o updater. |
| **QA & Test Specialist** | `self` | Diseño e implementación de suites de pruebas unitarias, mocks y tests de integración. | Cuando se requiera alcanzar cobertura de testing o validar casos borde complejos. |
| **Technical Documentation Specialist** | `self` | Redacción de documentación y especificaciones técnicas (`docs/`, `README.md`, diagramas Mermaid). | Para generar o sincronizar documentación tras la implementación de funcionalidades. |

---

## 4. Convenciones de Código y Testing

### 4.1. Estructura de Directorios y Arquitectura de Capas

```text
gz-ia/
├── .forgejo/
│   └── workflows/
│       ├── release.yml         # Pipeline CI/CD para compilar y publicar releases en Nexus
│       └── docs.yml            # Pipeline CI/CD para desplegar la documentación en Cloudflare Pages
├── cmd/
│   └── gz-ia/
│       ├── main.go             # Punto de entrada de la aplicación
│       └── main_test.go        # Test de ejecución
├── docs/                       # Subproyecto de documentación VitePress (Guía, Arquitectura, Orchy, Referencia)
├── internal/
│   ├── clients/                # Mecanismos de entrega y presentación (clientes)
│   │   ├── cli/                # Cliente Cobra (root, chat, session, start, version, update, cli_test)
│   │   └── tui/                # Cliente interactivo Bubble Tea / Huh (tui, banner, theme, tests)
│   ├── features/               # Núcleo funcional y casos de uso
│   │   ├── session/            # Ciclo de vida de sesiones (service, session, store, killer, drivers, tests)
│   │   ├── workspace/          # Aprovisionamiento y aislamiento Git (Provider, Worktrees, Diff, Merge, tests)
│   │   ├── metrics/            # Telemetría, tokens y trazado recursivo de subagentes (model, collector, service, tests)
│   │   ├── logger/             # Observabilidad de estados y log de eventos por sesión (.events.jsonl, model, store, service, tests)
│   │   └── updater/            # Auto-actualizador contra Forgejo y Sonatype Nexus (model, service, tests)
│   └── version/                # Metadatos de versión y compilación
├── packages/
│   └── orchy/                  # Proveedor de Microkernel reactivo y servidor MCP para gz-ia
│       ├── core/               # Kernel, KernelContext, ServiceContainer (IoC thread-safe)
│       ├── events/             # EventBus desacoplado (Pub/Sub y Request-Response RPC con context)
│       ├── tools/              # Tool, ToolProxy (Circuit Breaker Honest Kernel), ToolRegistry
│       ├── plugins/            # Interfaz Plugin, BasePlugin y PluginRegistry de ciclo de vida
│       ├── batteries/          # Plugins nativos (worktree_read, worktree_get)
│       ├── mcp/                # ManifestGenerator, JSON-RPC 2.0 y servidor Stdio MCP
│       └── orchy.go            # Facade raíz del paquete con constructores ergonómicos
├── .harness/
│   └── sessions/               # Registros JSON de sesiones (.json) y eventos (.events.jsonl)
├── AGENTS.md                   # Este documento de convenciones y roles
└── Makefile                    # Automatización de tareas de compilación y testing
```

#### Arquitectura de Capas Desacopladas

```mermaid
flowchart TD
    subgraph Delivery["Mecanismos de Entrega (Clientes: internal/clients/)"]
        CLI["internal/clients/cli<br/>(Cobra CLI Commands)"]
        TUI["internal/clients/tui<br/>(Terminal User Interface)"]
    end

    subgraph Features["Núcleo Funcional y Casos de Uso (internal/features/)"]
        FeatService["internal/features/session<br/>(Service, Session, Store, ProcessKiller)"]
        WS["internal/features/workspace<br/>(Provider, Worktree, Diff, Merge)"]
        Met["internal/features/metrics<br/>(Collector, Telemetry, Tokens, Subagents)"]
        Log["internal/features/logger<br/>(Service, Store, Events JSONL, Watch)"]
        Upd["internal/features/updater<br/>(Service, CheckLatest, Update, Atomic Replacement)"]
    end

    CLI -->|consume| FeatService
    CLI -->|consume| Upd
    TUI -->|consume| FeatService
    FeatService -->|orquesta| WS
    FeatService -->|orquesta| Met
    FeatService -->|orquesta| Log
```

- **Mecanismos de Entrega (`internal/clients/cli`, `internal/clients/tui`):** Actúan estrictamente como clientes/adaptadores. Se limitan a interactuar con el usuario, parsear argumentos o capturar entradas interactivas, invocar `features/session.Service` o `features/updater.Service` y formatear las salidas visuales o errores.
  - `internal/clients/cli`: Provee el subcomando `chat` con la bandera `-p, --provider` (default: primer disponible o `agy`), validando la presencia del agente en `$PATH` y emitiendo sugerencias de instalación (`InstallHint()`) en caso de ausencia.
  - `internal/clients/tui`: Ofrece una experiencia accesible guiada mediante `handleNewChat`, presentando detección de estado visual (`[✓]` / `[✗]`), preselección inteligente hacia el primer agente listo (o `[←] Volver al menú principal` si ninguno está instalado), validación bloqueante con retroalimentación en pantalla y enlaces de retorno explícitos en cada vista.
- **Núcleo Funcional y Casos de Uso (`internal/features/`):**
  - `internal/features/session`: Expone la interfaz `Service` unificada conteniendo la lógica de ciclo de vida de sesiones, ejecución desacoplada (`Runner`), persistencia atómica en disco (`Store`), terminación POSIX (`ProcessKiller`), métricas, logger y la **Capa de Drivers Multi-Proveedor** (`driver.go`):
    - Interfaz `Driver`: `ID()`, `DisplayName()`, `BinaryName()`, `IsAvailable()`, `InstallHint()`, `BuildArgs(cfg)`.
    - Drivers soportados: Google Antigravity (`agy`), Claude Code (`claude`), OpenCode (`opencode`) y Pi Agent (`pi-agent`).
    - Registro central: `ListDrivers()`, `GetDriver(id)` y `FirstAvailableDriver()`.
    - Persistencia en `SessionRecord`: metadata con `provider` para permitir reanudación automática (`Resume`) invocando al agente nativo correspondiente.
  - `internal/features/workspace`: Aprovisionamiento y aislamiento en Git Worktrees (`Provider`), cálculo de diffs, integración merge y detección de raíz de proyectos.
  - `internal/features/metrics`: Recolector portable de telemetría de Antigravity (`history.jsonl`, `transcript.jsonl`), cálculo de duraciones, desglose de tokens (~4 caracteres/token para prompt, thinking y completion), conteo de llamadas a herramientas y rastreo recursivo de subagentes (`invoke_subagent`).
  - `internal/features/logger`: Registro seguro y concurrente de eventos de observabilidad (`.harness/sessions/<id>.events.jsonl`), transiciones de etapas (`READ` -> `PENDING` -> `FINISH`), estados y transmisión reactiva en tiempo real (`Watch`).
  - `internal/features/updater`: Verificación de versiones más recientes en Forgejo Tags API y descarga atómica y segura de binarios desde Sonatype Nexus (`CheckLatest`, `Update`).
- **Metadatos (`internal/version`):** Variables de compilación (`Version`, `Commit`, `Date`).

### 4.2. Convenciones de Observabilidad para Agentes y Subagentes (`session log`)

Para asegurar la trazabilidad continua y el monitoreo en tiempo real del progreso sin saturar los canales principales de interacción:

1. **Emisión Obligatoria de Eventos:**
   Tanto el agente orquestador como los subagentes delegados deben notificar hitos clave de su ejecución mediante `gz-ia session log <id>`:
   ```bash
   gz-ia session log <id> -a "<acción descriptiva>" -s <READ|PENDING|FINISH> [--status <OK|FAILED>] [--agent <id>] [-r <rol>] [--duration <ms>] [--error "<err>"]
   ```

2. **Ciclo de Etapas Estandarizado:**
   - **`READ`:** Fase de exploración, lectura de archivos, búsqueda o inspección de contexto inicial. Estado: predeterminado `null`.
   - **`PENDING`:** Fase de ejecución activa, edición de código, refactorización o ejecución de pruebas intermedias. Estado: predeterminado `null`.
   - **`FINISH`:** Hito de culminación o entrega de una tarea. Debe acompañarse explícitamente de `--status OK` si finalizó con éxito o `--status FAILED` junto con `--error "<detalle>"` si se produjo un fallo.

3. **Ejemplo de Trazabilidad de un Subagente:**
   ```bash
   # Inicio de exploración
   gz-ia session log "$SESSION_ID" -a "Leyendo especificación y dependencias" -s READ --agent "sub-tester" -r "QA"

   # Ejecución activa
   gz-ia session log "$SESSION_ID" -a "Ejecutando suite de pruebas unitarias" -s PENDING --agent "sub-tester" -r "QA"

   # Culminación exitosa
   gz-ia session log "$SESSION_ID" -a "Pruebas concluidas al 100%" -s FINISH --status OK --agent "sub-tester" -r "QA" --duration 420
   ```

4. **Monitoreo en Vivo:**
   Los observadores, paneles o usuarios pueden seguir las transiciones en tiempo real ejecutando:
   ```bash
   gz-ia session logs <id> -f
   ```

5. **Actualización Automatizada:**
   El arnés puede verificar y actualizarse a la última versión disponible publicada en Nexus:
   ```bash
   gz-ia update --check
   gz-ia update
   ```

### 4.3. Estándares en Go
- **Constructores sin estado global:** Los comandos Cobra y clientes TUI deben instanciarse mediante funciones fábrica (`NewRootCmd() *cobra.Command`, `NewService(...) session.Service`, `tui.New(...)`) para permitir pruebas unitarias concurrentes y limpias.
- **Inyección de dependencias:** La capa de features y los clientes reciben abstracciones (`Runner`, `Store`, `ProcessKiller`, `workspace.Provider`, `logger.Service`, `features/session.Service`) para permitir pruebas con mocks sin efectos secundarios.
- **Manejo de Errores Idiomático:** Retornar errores descriptivos con contexto (`fmt.Errorf("falló al preparar workspace: %w", err)`).
- **Formato:** Todo el código debe estar formateado con `gofmt` estándar.

### 4.4. Arquitectura del Microkernel Orchy (`packages/orchy`)

`packages/orchy` es el paquete proveedor de microkernel para `gz-ia`, diseñado siguiendo el patrón de arquitectura desacoplada reactiva con **Circuit Breaker** (Honest Kernel). Permite orquestar herramientas modulares, plugins extensibles, mensajería interna desacoplada y exposición nativa al protocolo **MCP (Model Context Protocol)** por Stdio.

```mermaid
flowchart TD
    subgraph OrchyKernel["Microkernel Orchy (packages/orchy)"]
        K["core.Kernel<br/>(Ciclo de Vida: IDLE -> BOOTING -> RUNNING -> STOPPED)"]
        Ctx["core.KernelContext<br/>(Bus de Ejecución)"]
        IoC["core.ServiceContainer<br/>(IoC Thread-Safe)"]
        EB["events.EventBus<br/>(Pub/Sub & RPC con Context)"]
        TR["tools.ToolRegistry<br/>(Catálogo de Herramientas)"]
        PR["plugins.PluginRegistry<br/>(Plugins & Extensiones)"]
        CB["tools.ToolProxy<br/>(Circuit Breaker: HEALTHY / DEGRADED / DEAD)"]
    end

    subgraph MCP["Protocolo MCP (packages/orchy/mcp)"]
        MG["mcp.ManifestGenerator<br/>(Introspección & JSON Schema)"]
        Srv["mcp.Server<br/>(JSON-RPC 2.0 Stdio)"]
    end

    K --> Ctx
    Ctx --> IoC
    Ctx --> EB
    Ctx --> TR
    TR --> CB
    K --> PR
    K --> MG
    Srv --> MG
    Srv --> Ctx
```

#### Principios Clave de Orchy:
1. **Honest Kernel & Circuit Breaker (`tools.ToolProxy`):**
   - Cada herramienta se envuelve automáticamente en un `ToolProxy` que monitorea fallas consecutivas y tiempos de respuesta.
   - Estado inicial: `HEALTHY`.
   - Si una herramienta falla $\ge 2$ veces consecutivas: entra en `DEGRADED`.
   - Si supera `MaxConsecutiveFailures` (default 5): pasa a `DEAD` y sus llamadas futuras se rechazan inmediatamente en memoria para aislar la falla y no tumbar el servidor MCP ni saturar procesos del sistema.
   - Si la herramienta ejecuta exitosamente mientras está `DEGRADED`: se recupera automáticamente a `HEALTHY`.
   - El método `ResetCircuitBreaker()` permite restablecer manualmente su estado.
2. **Exclusión de Herramientas Muertas (`mcp.ManifestGenerator`):**
   - El catálogo de herramientas vivas expuesto a la IA mediante `tools/list` omite automáticamente las herramientas en estado `DEAD`, evitando que el modelo de IA genere bucles infinitos llamando a herramientas dañadas.
3. **EventBus Desacoplado con RPC (`events.EventBus`):**
   - Permite Pub/Sub asíncrono con cancelación por `context.Context` (`Publish`, `Subscribe`).
   - Soporta RPC síncrono desacoplado (`Request`, `Respond`) con control de timeouts.
4. **Servidor MCP Stdio (`mcp.Server`):**
   - Protocolo estándar JSON-RPC 2.0 compatible con clientes de IA, procesando `initialize`, `tools/list` y `tools/call` línea por línea en streams `io.Reader` e `io.Writer`.

### 4.5. Baterías de Worktree y Paridad Operativa (`packages/orchy/batteries/worktree`)

Para garantizar la paridad operacional entre la interfaz de usuario humana (CLI y TUI) y las capacidades de los agentes de IA (MCP):
1. **Herramientas MCP Nativas (`packages/orchy/batteries/worktree`):**
   - `worktree_read` (`packages/orchy/batteries/worktree/tool_read.go`): Inspección segura de diff y archivos modificados en un worktree de sesión sin tocar el workspace base.
   - `worktree_get` (`packages/orchy/batteries/worktree/tool_get.go`): Fusión e integración controlada de cambios del worktree hacia la rama de trabajo principal (`session.Service.Get`).
2. **Paridad Total en CLI:**
   - `gz-ia session read <id> [--stat]`: Equivalente CLI directo de `worktree_read`.
   - `gz-ia session get <id> [--no-commit] [--squash]`: Equivalente CLI directo de `worktree_get`.
3. **Paridad en TUI:**
   - La vista de detalle de sesión (`handleSessionDetail`) incluye las acciones interactivas `read` (con visor estilizado en Lipgloss) y `get` (con diálogo de confirmación y resumen de archivos integrados).

---

## 5. Protocolo de Comunicación con el Usuario

- Respuestas breves, directas y en Markdown al estilo GitHub.
- Referencias limpias a rutas relativas del proyecto (`internal/features/...`) y símbolos de código.
- Consultar siempre al usuario ante decisiones de diseño que impliquen trade-offs importantes.
- Proponer siempre el siguiente paso inmediato con opciones claras y sin suposiciones no verificadas.
