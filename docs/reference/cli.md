# Referencia Completa del CLI

Guía detallada de sintaxis, argumentos, banderas y códigos de retorno de todos los comandos y subcomandos de `gz-ia`.

---

## Tabla Resumen de Comandos

| Comando / Subcomando | Argumentos / Flags | Nivel de Permiso | Descripción |
| :--- | :--- | :--- | :--- |
| [`gz-ia`](#gz-ia-start) | N/A | Heredado | Lanza la interfaz gráfica de terminal (TUI) por defecto. |
| [`gz-ia start`](#gz-ia-start) | N/A | Heredado | Inicia la TUI interactiva con estética Fastfetch y menús. |
| [`gz-ia chat`](#gz-ia-chat) | `-p, --provider`<br>`-m, --perm`<br>`-T, --toolkit`<br>`-P, --preset, --profile`<br>`-i, --prompt`<br>`-d, --dir` | `readonly`<br>`supervised`<br>`autonomous` | Inicia una sesión directa conectada al agente de IA seleccionado. |
| [`gz-ia toolkit list`](#gz-ia-toolkit) | `-d, --dir` | N/A | Lista los toolkits modulares y presets disponibles (globales y de proyecto). |
| [`gz-ia toolkit show`](#gz-ia-toolkit) | `<id>`<br>`-d, --dir` | N/A | Muestra la configuración detallada de un toolkit o preset. |
| [`gz-ia toolkit create`](#gz-ia-toolkit) | `<id>`<br>`--desc`<br>`--global`<br>`-d, --dir` | N/A | Crea el scaffolding inicial para un nuevo toolkit modular (alias: `init`). |
| [`gz-ia toolkit skills`](#gz-ia-toolkit) | `-d, --dir` | N/A | Explora el catálogo unificado de skills disponibles en los toolkits. |
| [`gz-ia toolkit path`](#gz-ia-toolkit) | `[id]`<br>`-d, --dir` | N/A | Imprime la ruta al directorio del toolkit o del catálogo local. |
| [`gz-ia session list`](#gz-ia-session-list) | `-d, --dir` | N/A | Lista en formato tabular todas las sesiones del proyecto. |
| [`gz-ia session show`](#gz-ia-session-show) | `<id>`<br>`-d, --dir` | N/A | Muestra la metadata detallada y estado de una sesión. |
| [`gz-ia session kill`](#gz-ia-session-kill) | `<id>`<br>`-d, --dir` | N/A | Termina de forma controlada el proceso de una sesión (`SIGTERM` + `SIGKILL` en Unix / `taskkill` en Windows). |
| [`gz-ia session resume`](#gz-ia-session-resume) | `<id>`<br>`-r, --reload-toolkits`<br>`-d, --dir` | Heredado de la sesión | Reanuda una sesión previa reconectando al agente nativo (`--continue`/`--resume`), re-proyectando opcionalmente toolkits. |
| [`gz-ia session reload-toolkits`](#gz-ia-session-reload-toolkits) | `<id>`<br>`-d, --dir` | N/A | Recarga en caliente los manifiestos, directivas, reglas y skills de los toolkits en la sesión. |
| [`gz-ia session delete`](#gz-ia-session-delete) | `<id>`<br>`-d, --dir` | N/A | Elimina el registro persistente y destruye el worktree y la rama asociada. |
| [`gz-ia session path`](#gz-ia-session-path) | `<id>`<br>`-d, --dir` | N/A | Imprime en `stdout` la ruta absoluta del espacio de trabajo. |
| [`gz-ia session diff`](#gz-ia-session-diff) | `<id>`<br>`--stat`<br>`-d, --dir` | N/A | Muestra las diferencias de código producidas en la sesión. |
| [`gz-ia session read`](#gz-ia-session-read) | `<id>`<br>`--stat`<br>`-d, --dir` | N/A | Inspecciona el contenido y diff actual del worktree sin mutar el base. |
| [`gz-ia session get`](#gz-ia-session-get) | `<id>`<br>`-d, --dir` | N/A | Trae las modificaciones del worktree hacia el directorio activo como unstaged sin git merge commits. |
| [`gz-ia session context`](#gz-ia-session-context) | `<id>`<br>`-j, --json`<br>`-d, --dir` | N/A | Muestra el contexto y estado de trabajo de una sesión (handoff para agentes). |
| [`gz-ia session metrics`](#gz-ia-session-metrics) | `<id>`<br>`--json`<br>`--detailed`<br>`-d, --dir` | N/A | Calcula duración, pasos, desglose de tokens y uso de herramientas. |
| [`gz-ia session log`](#gz-ia-session-log) | `<id>`<br>`-a, --action`<br>`-s, --stage`<br>`--status`<br>`--agent`<br>`-r, --role`<br>`--duration`<br>`--error`<br>`-d, --dir` | N/A | Registra un evento estructurado en `.events.jsonl`. |
| [`gz-ia session logs`](#gz-ia-session-logs) | `<id>`<br>`-f, --follow`<br>`--json`<br>`-d, --dir` | N/A | Consulta histórica o transmisión en vivo de los eventos de una sesión. |
| [`gz-ia session prune`](#gz-ia-session-prune) | `-d, --dir` | N/A | Reconcilia y elimina sesiones, ramas de Git y worktrees huérfanos. |
| [`gz-ia memory`](#gz-ia-memory) | `save`, `search`, `list`, `consolidate`<br>`-d, --dir` | N/A | Almacena y consulta memoria semántica local con ranking Okapi BM25. |
| [`gz-ia vault`](#gz-ia-vault) | `list`, `set`, `get`, `delete`, `path`<br>`-d, --dir` | N/A | Gestiona secretos y variables de entorno centralizadas (`.harness/vault.json`). |
| [`gz-ia mcp`](#gz-ia-mcp) | `--session`<br>`--tooling`<br>`--profile`<br>`--toolkit`<br>`--allow-get`<br>`-d, --dir` | N/A | Inicia el servidor MCP nativo de gz-ia sobre Stdio (JSON-RPC 2.0). |
| [`gz-ia update`](#gz-ia-update) | `-c, --check`<br>`-f, --force`<br>`--version`<br>`--install-dir` | N/A | Verifica e instala la última versión de `gz-ia` desde GitHub Releases. |
| [`gz-ia version`](#gz-ia-version) | N/A | N/A | Muestra la versión, commit SHA y fecha de compilación. |

---

## Detalle de Comandos

### <span id="gz-ia-start"></span>`gz-ia` / `gz-ia start`

Inicia la interfaz de usuario de terminal (TUI interactiva).

```bash
gz-ia
# o
gz-ia start
```

---

### `gz-ia chat`

Lanza una sesión de terminal conectada a un agente de IA.

```bash
gz-ia chat [flags]
```

**Banderas (Flags):**
- `-p, --provider <id>`: Proveedor agéntico (`agy`, `claude`, `opencode`, `pi-agent`). Predeterminado: primer agente instalado disponible.
- `-m, --perm <nivel>`: Nivel de permiso (`readonly`, `supervised`, `autonomous`). Predeterminado: `supervised`.
- `-T, --toolkit <id>`: Toolkits modulares a activar, separados por coma o repetidos (ej. `-T rn-bridge` o `-T tk1 -T tk2`).
- `-P, --preset, --profile <nombre>`: Presets o perfiles agénticos a activar, separados por coma o repetidos (ej. `-P fullstack` o `-P frontend,data`). `--profile` se mantiene como alias de compatibilidad.
- `-i, --prompt <texto>`: Prompt inicial inyectado al agente de IA.
- `-d, --dir <ruta>`: Directorio de trabajo base (predeterminado: directorio actual).

**Ejemplos:**
```bash
# Chat supervisado estándar
gz-ia chat -p agy

# Chat activando toolkits modulares específicos (-T)
gz-ia chat -p agy -T frontend -T backend

# Chat con un preset de capacidades (-P)
gz-ia chat -p agy -P fullstack

# Chat autónomo combinando preset, toolkit modular y prompt inicial
gz-ia chat -p claude -m autonomous -P fullstack -T e2e-testing -i "Refactorizar internal/logger"
```

---

### `gz-ia toolkit`

Gestiona toolkits modulares, presets agénticos y explora el catálogo unificado de skills.  
*Aliases aceptados:* `toolkits`, `profile`, `profiles`.

#### `gz-ia toolkit list`
Lista todos los toolkits modulares registrados y presets agénticos disponibles (combinando ámbito global `~/.config/gz-ia/tooling` y de proyecto `.gz-ia/toolkits` o `toolkits/`). Muestra la cantidad de skills, tools y su ámbito.

```bash
gz-ia toolkit list [-d <directorio>]
```

#### `gz-ia toolkit show <id>`
Muestra la definición detallada de un toolkit o preset: identificador, descripción, ámbito, directivas (`AGENTS.md`), reglas (`rules/`), skills, herramientas MCP ejecutables (`tools.json`) y servidores MCP asociados.

```bash
gz-ia toolkit show <id> [-d <directorio>]
```

#### `gz-ia toolkit create <id>` (alias: `init`)
Crea el scaffolding inicial para un nuevo toolkit modular con su estructura estándar: `toolkit.json`, directivas maestras (`AGENTS.md`), regla de ejemplo (`rules/example.md`), habilidad de ejemplo (`skills/example/SKILL.md`) y declaración de herramienta (`tools.json`).

```bash
gz-ia toolkit create <id> [--desc <descripción>] [--global] [-d <directorio>]
# o mediante su alias:
gz-ia toolkit init <id> [--desc <descripción>] [--global] [-d <directorio>]
```

**Banderas:**
- `--desc <texto>`: Descripción legible del toolkit modular.
- `--global`: Crea el toolkit en el catálogo global de usuario (`~/.config/gz-ia/tooling/toolkits/<id>`) en lugar del proyecto actual (`.gz-ia/toolkits/<id>`).
- `-d, --dir <ruta>`: Directorio del proyecto de destino.

#### `gz-ia toolkit skills`
Explora todas las habilidades descubiertas en los toolkits disponibles en el entorno, detallando su nombre, categoría inferida por prefijo (`front-*`, `data-*`, `back-*`, `devops-*`), toolkit contenedor, ámbito y descripción.

```bash
gz-ia toolkit skills [-d <directorio>]
```

#### `gz-ia toolkit path [id]`
Imprime en `stdout` la ruta absoluta al directorio del toolkit especificado o al catálogo de toolkits del proyecto (`.gz-ia/toolkits`). Diseñado para scripting limpio de shell (`cd $(gz-ia toolkit path frontend)`).

```bash
gz-ia toolkit path [id] [-d <directorio>]
```

---

### `gz-ia session list`

Lista en formato tabular todas las sesiones agénticas del proyecto.

```bash
gz-ia session list [-d <ruta>]
```

---

### `gz-ia session show`

Muestra la metadata en detalle de una sesión específica.

```bash
gz-ia session show <session-id> [-d <ruta>]
```

---

### `gz-ia session kill`

Detiene forzosamente el proceso del agente de la sesión.

```bash
gz-ia session kill <session-id> [-d <ruta>]
```

Envía `SIGTERM` al grupo de procesos y escala a `SIGKILL` si no finaliza dentro del tiempo de espera (en Windows ejecuta `taskkill /F /T` para terminar el árbol de procesos).

---

### <span id="gz-ia-session-resume"></span>`gz-ia session resume` *(alias: `continue`)*

Reanuda una sesión existente, reingresando a su worktree y reconectando la terminal interactiva con la bandera de reanudación adecuada del agente (`--continue` o `--resume`). Con la opción `--reload-toolkits` (`-r`), fuerza adicionalmente la resolución, composición y re-proyección atómica de reglas, habilidades y manifiestos MCP en el espacio de trabajo antes de reanudar la ejecución.

```bash
gz-ia session resume <session-id> [--reload-toolkits | -r]
gz-ia session continue <session-id> [-r]
```

#### Flags

- `-r, --reload-toolkits`: Fuerza la recarga, resolución y re-proyección de los toolkits asignados a la sesión en el worktree (o directorio activo) antes de reanudar la ejecución del agente.

---

### <span id="gz-ia-session-reload-toolkits"></span>`gz-ia session reload-toolkits`

Recarga activamente los manifiestos, directivas (`AGENTS.md`), reglas (`.agents/rules/`), habilidades (`.agents/skills/`) y servidores MCP de los toolkits asignados a una sesión en su espacio de trabajo o worktree aislado sin necesidad de detener ni reiniciar la sesión.

```bash
gz-ia session reload-toolkits <session-id> [-d <directorio>]
```

#### Flags

- `-d, --dir <directorio>`: Ruta al directorio del proyecto.

#### Ejemplo

```bash
gz-ia session reload-toolkits 3f9a12c8
```

---

### <span id="gz-ia-session-delete"></span>`gz-ia session delete` *(alias: `rm`)*

Elimina el registro de la sesión y destruye el Git Worktree efímero y la rama de Git correspondiente.

```bash
gz-ia session delete <session-id>
gz-ia session rm <session-id>
```

---

### `gz-ia session path`

Imprime únicamente la ruta absoluta del directorio de trabajo de la sesión.

```bash
gz-ia session path <session-id>
```

---

### `gz-ia session diff`

Muestra las diferencias Git acumuladas en la sesión.

```bash
gz-ia session diff <session-id> [--stat]
```

---

### `gz-ia session read`

Inspecciona el contenido y diff actual de la sesión de manera estructurada (compatible con herramientas agénticas).

```bash
gz-ia session read <session-id> [--stat]
```

---

### <span id="gz-ia-session-get"></span>`gz-ia session get` *(alias: `merge`)*

Trae las modificaciones y archivos creados desde el worktree de sesión hacia el directorio de trabajo activo.

```bash
gz-ia session get <session-id> [-d <directorio>]
```

**Semántica técnica de integración:**
1. Trae todos los cambios y archivos nuevos producidos en la sesión directamente al directorio de trabajo activo como modificaciones no preparadas (*unstaged*).
2. No realiza `git merge` ni crea commits automáticos en el repositorio principal, preservando la soberanía absoluta de Git y evitando estados intermediarios o conflictos de merge.
3. Si no existen cambios pendientes en la sesión, informa que el directorio ya se encuentra actualizado.

---

### <span id="gz-ia-session-context"></span>`gz-ia session context`

Muestra el contexto unificado y el estado de trabajo de una sesión, diseñado para handoff y continuidad entre agentes.

```bash
gz-ia session context <session-id> [flags]
```

**Banderas (Flags):**
- `-j, --json`: Emite la estructura completa de contexto en formato JSON a `stdout`.
- `-d, --dir <ruta>`: Directorio del proyecto.

**Contenido del contexto:**
- Metadatos de la sesión: ID, `conversation_id` (persistido del proveedor agéntico), proveedor, rama, toolkits, prompt inicial y estado.
- Métricas temporales: fecha de inicio, finalización (`FinishedAt`) y duración en milisegundos (`DurationMs`) garantizadas por `defer`.
- Historial de eventos de observabilidad (`.events.jsonl`): acciones, etapas (`READ`, `PENDING`, `FINISH`), duraciones, estados y eventos de telemetría sincronizados automáticamente de `transcript.jsonl` (`SyncTelemetry`).
- Auditoría delta de archivos modificados y creados en el worktree (`UpdateManifestDelta` en `manifest.json`).

**Ejemplos:**
```bash
# Inspección visual del contexto de sesión
gz-ia session context 3f9a12c8

# Exportación de contexto en formato JSON para consumo de otro agente
gz-ia session context 3f9a12c8 --json
```

---

### `gz-ia session metrics`

Calcula y presenta el reporte de métricas de la sesión.

```bash
gz-ia session metrics <session-id> [--json] [--detailed]
```

---

### `gz-ia session log`

Registra un evento de observabilidad en `.harness/sessions/<id>.events.jsonl`.

```bash
gz-ia session log <session-id> \
  -a, --action <texto> \
  -s, --stage <READ|PENDING|FINISH> \
  --status <OK|FAILED> \
  --agent <nombre> \
  [-r, --role <rol>] \
  [--duration <milisegundos>] \
  [--error <mensaje_error>]
```

---

### `gz-ia session logs`

Visualiza o transmite el flujo de eventos de la sesión.

```bash
gz-ia session logs <session-id> [-f, --follow] [--json]
```

Los eventos consultados provienen de `.harness/sessions/<id>.events.jsonl` e incluyen:
1. **Eventos manuales/directos:** Registrados vía `session log` o por la TUI/orquestador.
2. **Eventos de telemetría sincronizada (`SyncTelemetry`):** Extraídos de `transcript.jsonl` (llamadas a herramientas y subagentes invocados).
3. **Evento de Cierre Garantizado (`StageFinish`):** Generado por el bloque `defer` al finalizar la sesión con `exit_code`, `duration_s` y el diff de archivos modificados.

---

### `gz-ia session prune`

Reconcilia y elimina sesiones, ramas de Git y worktrees huérfanos que hayan quedado desfasados por eliminaciones manuales o interrupciones.

```bash
gz-ia session prune [-d, --dir <directorio>]
```

---

### <span id="gz-ia-memory"></span>`gz-ia memory`

Gestiona memoria semántica local y contexto de decisiones (arquitectura, reglas de negocio, hallazgos) con ranking semántico Okapi BM25.

#### `gz-ia memory save`
Guarda un registro de memoria o regla de conocimiento.

```bash
gz-ia memory save -t <título> -c <contenido> [flags]
```

**Banderas:**
- `-t, --title <texto>`: Título descriptivo de la memoria (obligatorio).
- `-c, --content <texto>`: Contenido detallado, código o especificación (obligatorio).
- `--category <categoría>`: Categoría opcional (`decision`, `rule`, `architecture`, `bugfix`).
- `--tags <etiquetas>`: Etiquetas separadas por coma para filtrado.
- `--session <id>`: ID de sesión para guardar en la memoria local de sesión (`.harness/sessions/<id>.memory.json`). Si se omite, se guarda en la memoria global del proyecto (`.harness/memory.json`).
- `-d, --dir <ruta>`: Directorio del proyecto.

#### `gz-ia memory search <query>`
Busca memorias relevantes ordenadas por el algoritmo de ranking Okapi BM25 combinando los almacenes de sesión y proyecto.

```bash
gz-ia memory search <consulta> [flags]
```

**Banderas:**
- `--session <id>`: ID de sesión para incluir su memoria local en la búsqueda.
- `--global-only`: Busca exclusivamente en la memoria del proyecto.
- `-l, --limit <n>`: Límite de resultados (predeterminado: 10).
- `-d, --dir <ruta>`: Directorio del proyecto.

#### `gz-ia memory list`
Lista los registros de memoria guardados.

```bash
gz-ia memory list [flags]
```

**Banderas:**
- `--session <id>`: ID de sesión para incluir memorias locales.
- `--global-only`: Lista exclusivamente memorias globales del proyecto.
- `-j, --json`: Emite el catálogo completo de registros en formato JSON.
- `-d, --dir <ruta>`: Directorio del proyecto.

#### `gz-ia memory consolidate <session_id>`
Promueve las entradas de memoria de una sesión específica hacia el almacén global del proyecto (`.harness/memory.json`).

```bash
gz-ia memory consolidate <session_id> [-d <ruta>]
```

---

### `gz-ia vault`

Gestiona secretos y variables de entorno centralizadas para perfiles y agentes con almacenamiento seguro local en `.harness/vault.json` (permisos estrictos `0600` e ignorado automáticamente por Git).

#### Subcomandos:

```bash
# Listar variables configuradas y recomendadas con valores ofuscados
gz-ia vault list [-d, --dir <directorio>]

# Guardar o actualizar una variable (si omites el valor, se solicita de forma interactiva y enmascarada)
gz-ia vault set <VARIABLE> [VALOR] [-v, --value <val>] [-d, --dir <directorio>]

# Inspeccionar el estado y origen de una variable (ofuscada por defecto o visible con --reveal)
gz-ia vault get <VARIABLE> [--reveal] [-d, --dir <directorio>]

# Eliminar una variable del vault (aliases: rm, remove)
gz-ia vault delete <VARIABLE> [-d, --dir <directorio>]
gz-ia vault rm <VARIABLE>
gz-ia vault remove <VARIABLE>

# Imprimir la ruta física absoluta del archivo vault.json
gz-ia vault path [-d, --dir <directorio>]
```

---

### `gz-ia mcp`

Inicia el servidor MCP nativo de `gz-ia` sobre Stdio (JSON-RPC 2.0).

El arnés actúa como su propio servidor MCP para agentes de IA como Google Antigravity (`agy`) o Claude Code (`claude`), exponiendo herramientas del microkernel Orchy (como `worktree_read`) y las herramientas de los toolkits declarados en `~/.config/gz-ia/tooling/`.

```bash
gz-ia mcp [flags]
```

**Banderas (Flags):**
- `--session <id>`: Identificador de la sesión de trabajo activa. Si la sesión es aislada, opera en su worktree `.harness/worktrees/<id>`.
- `--tooling <ruta>`: Directorio de tooling global (por defecto `~/.config/gz-ia/tooling`).
- `-P, --preset, --profile <nombre>`: Preset o perfil agéntico de tooling a activar.
- `--toolkit <id>`: Toolkits específicos a incorporar al microkernel.
- `--allow-get`: Permite exponer la herramienta `worktree_get` al agente (por defecto `false` por seguridad).
- `-d, --dir <ruta>`: Directorio de trabajo base.

---

### `gz-ia update`

Verifica e instala la última versión binaria disponible desde GitHub Releases.

```bash
gz-ia update [flags]
```

**Banderas:**
- `-c, --check`: Comprueba únicamente si existe una versión más reciente sin descargarla.
- `-f, --force`: Fuerza la reinstalación o downgrade a la versión indicada.
- `--version <v>`: Especifica la versión exacta a instalar (ej. `0.0.2`).
- `--install-dir <dir>`: Ruta donde se ubicará el binario actualizado.

---

### `gz-ia version`

Muestra la información de versión del binario compilado.

```bash
gz-ia version
```
