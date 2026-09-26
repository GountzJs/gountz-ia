# Referencia Completa del CLI

Guía detallada de sintaxis, argumentos, banderas y códigos de retorno de todos los comandos y subcomandos de `gz-ia`.

---

## Tabla Resumen de Comandos

| Comando / Subcomando | Argumentos / Flags | Nivel de Permiso | Descripción |
| :--- | :--- | :--- | :--- |
| [`gz-ia`](#gz-ia-start) | N/A | Heredado | Lanza la interfaz gráfica de terminal (TUI) por defecto. |
| [`gz-ia start`](#gz-ia-start) | N/A | Heredado | Inicia la TUI interactiva con estética Fastfetch y menús. |
| [`gz-ia chat`](#gz-ia-chat) | `-p, --provider`<br>`-m, --perm`<br>`-P, --profile`<br>`-i, --prompt`<br>`-d, --dir` | `readonly`<br>`supervised`<br>`autonomous` | Inicia una sesión directa conectada al agente de IA seleccionado. |
| [`gz-ia profile list`](#gz-ia-profile) | `-d, --dir` | N/A | Lista los perfiles agénticos disponibles (globales y de proyecto). |
| [`gz-ia profile show`](#gz-ia-profile) | `<name>`<br>`-d, --dir` | N/A | Muestra la configuración detallada de un perfil agéntico. |
| [`gz-ia profile create`](#gz-ia-profile) | `<name>`<br>`--desc`<br>`--agents-file`<br>`--global`<br>`-d, --dir` | N/A | Crea un nuevo perfil agéntico con descriptor `perfil.json`. |
| [`gz-ia profile skills`](#gz-ia-profile) | `-d, --dir` | N/A | Explora el catálogo clasificado de skills disponibles. |
| [`gz-ia profile path`](#gz-ia-profile) | `[name]`<br>`-d, --dir` | N/A | Imprime la ruta al directorio del perfil o del catálogo. |
| [`gz-ia session list`](#gz-ia-session-list) | `-d, --dir` | N/A | Lista en formato tabular todas las sesiones del proyecto. |
| [`gz-ia session show`](#gz-ia-session-show) | `<id>`<br>`-d, --dir` | N/A | Muestra la metadata detallada y estado de una sesión. |
| [`gz-ia session kill`](#gz-ia-session-kill) | `<id>`<br>`-d, --dir` | N/A | Termina de forma controlada el proceso de una sesión (`SIGTERM` + `SIGKILL`). |
| [`gz-ia session resume`](#gz-ia-session-resume) | `<id>`<br>`-d, --dir` | Heredado de la sesión | Reanuda una sesión previa invocando al agente nativo (`--continue`/`--resume`). |
| [`gz-ia session delete`](#gz-ia-session-delete) | `<id>`<br>`-d, --dir` | N/A | Elimina el registro persistente y destruye el worktree y la rama asociada. |
| [`gz-ia session path`](#gz-ia-session-path) | `<id>`<br>`-d, --dir` | N/A | Imprime en `stdout` la ruta absoluta del espacio de trabajo. |
| [`gz-ia session diff`](#gz-ia-session-diff) | `<id>`<br>`--stat`<br>`-d, --dir` | N/A | Muestra las diferencias de código producidas en la sesión. |
| [`gz-ia session read`](#gz-ia-session-read) | `<id>`<br>`--stat`<br>`-d, --dir` | N/A | Inspecciona el contenido y diff actual del worktree sin mutar el base. |
| [`gz-ia session merge`](#gz-ia-session-merge) | `<id>`<br>`--squash`<br>`--no-commit`<br>`-d, --dir` | N/A | Integra los cambios del worktree a la rama principal de trabajo. |
| [`gz-ia session get`](#gz-ia-session-get) | `<id>`<br>`--squash`<br>`--no-commit`<br>`-d, --dir` | N/A | Trae e integra las modificaciones producidas en el worktree hacia el directorio activo. |
| [`gz-ia session metrics`](#gz-ia-session-metrics) | `<id>`<br>`--json`<br>`--detailed`<br>`-d, --dir` | N/A | Calcula duración, pasos, desglose de tokens y uso de herramientas. |
| [`gz-ia session log`](#gz-ia-session-log) | `<id>`<br>`-a, --action`<br>`-s, --stage`<br>`--status`<br>`--agent`<br>`-r, --role`<br>`--duration`<br>`--error`<br>`-d, --dir` | N/A | Registra un evento estructurado en `.events.jsonl`. |
| [`gz-ia session logs`](#gz-ia-session-logs) | `<id>`<br>`-f, --follow`<br>`--json`<br>`-d, --dir` | N/A | Consulta histórica o transmisión en vivo de los eventos de una sesión. |
| [`gz-ia session prune`](#gz-ia-session-prune) | `-d, --dir` | N/A | Reconcilia y elimina sesiones, ramas de Git y worktrees huérfanos. |
| [`gz-ia vault`](#gz-ia-vault) | `list`, `set`, `get`, `delete`, `path`<br>`-d, --dir` | N/A | Gestiona secretos y variables de entorno centralizadas (`.harness/vault.json`). |
| [`gz-ia mcp`](#gz-ia-mcp) | `--session`<br>`--tooling`<br>`--profile`<br>`--toolkit`<br>`--allow-get`<br>`-d, --dir` | N/A | Inicia el servidor MCP nativo de gz-ia sobre Stdio (JSON-RPC 2.0). |
| [`gz-ia update`](#gz-ia-update) | `-c, --check`<br>`-f, --force`<br>`--version`<br>`--install-dir` | N/A | Verifica e instala la última versión de `gz-ia` desde GitHub Releases. |
| [`gz-ia version`](#gz-ia-version) | N/A | N/A | Muestra la versión, commit SHA y fecha de compilación. |

---

## Detalle de Comandos

### `gz-ia` / `gz-ia start`

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
- `-P, --profile <nombre>`: Perfiles agénticos a activar, separados por coma o repetidos (ej. `-P frontend,data` o `-P frontend -P data`).
- `-i, --prompt <texto>`: Prompt inicial inyectado al agente de IA.
- `-d, --dir <ruta>`: Directorio de trabajo base (predeterminado: directorio actual).

**Ejemplos:**
```bash
# Chat supervisado estándar
gz-ia chat -p agy

# Chat con perfil agéntico frontend activado
gz-ia chat -p agy -P frontend

# Chat autónomo con múltiples perfiles y prompt inicial
gz-ia chat -p claude -m autonomous -P frontend,data -i "Refactorizar internal/logger"
```

---

### `gz-ia profile`

Gestiona perfiles agénticos y explora el catálogo unificado de skills.

#### `gz-ia profile list`
Lista todos los perfiles agénticos disponibles (combinando ámbito global `~/.config/gz-ia/profiles` y proyecto `.harness/profiles`). Si detecta colisión de nombres entre ambos ámbitos, falla de inmediato para evitar comportamientos no deterministas.

```bash
gz-ia profile list [-d <directorio>]
```

#### `gz-ia profile show <name>`
Muestra la definición detallada del perfil, incluyendo descripción, archivo de directivas (`*-AGENTS.md`), lista de skills requeridas y servidores MCP asociados.

```bash
gz-ia profile show <nombre> [-d <directorio>]
```

#### `gz-ia profile create <name>`
Crea una nueva carpeta de perfil con un `perfil.json` preconfigurado y plantilla de archivo de directivas opcional.

```bash
gz-ia profile create <nombre> [--desc <descripción>] [--agents-file <archivo>] [--global] [-d <directorio>]
```

**Banderas:**
- `--desc <texto>`: Descripción legible del perfil agéntico.
- `--agents-file <archivo>`: Nombre del archivo de directivas específico (ej. `FRONT-AGENTS.md`, `DATA-AGENTS.md`).
- `--global`: Crea el perfil en `~/.config/gz-ia/profiles` en lugar de la carpeta del proyecto actual (`.harness/profiles`).

#### `gz-ia profile skills`
Explora todas las habilidades descubiertas en los catálogos global y de proyecto, agrupadas automáticamente por categoría extraída de su prefijo (`front-*`, `data-*`, `back-*`, `devops-*`).

```bash
gz-ia profile skills [-d <directorio>]
```

#### `gz-ia profile path [name]`
Imprime en `stdout` la ruta absoluta al directorio del perfil especificado o al catálogo de perfiles del proyecto. Salida limpia para scripts de shell (`cd $(gz-ia profile path frontend)`).

```bash
gz-ia profile path [nombre] [-d <directorio>]
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

Envía `SIGTERM` y escala a `SIGKILL` si no finaliza dentro del tiempo de espera.

---

### `gz-ia session resume` *(alias: `continue`)*

Reanuda una sesión existente, reingresando a su worktree y reconectando la terminal interactiva con la bandera de reanudación adecuada del agente (`--continue` o `--resume`).

```bash
gz-ia session resume <session-id>
gz-ia session continue <session-id>
```

---

### `gz-ia session delete` *(alias: `rm`)*

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

### `gz-ia session get` *(alias: `merge`)*

Fusiona e integra los cambios del worktree hacia la rama de trabajo activa.

```bash
gz-ia session get <session-id> [--squash] [--no-commit]
```

**Banderas:**
- `--squash`: Condensa todos los commits intermedios de la sesión en un solo commit.
- `--no-commit`: Fusiona los archivos y los deja en el *staging area* de Git sin crear commit automático.

**Semántica técnica de integración:**
1. Si hay cambios pendientes en el worktree de la sesión, genera un commit de seguridad en la rama `harness/<id>`.
2. Ejecuta un `git merge` (o `git merge --squash` con `--squash`, y sin commit si se pasa `--no-commit`) de la rama `harness/<id>` en la rama activa del repositorio.
3. Si la rama base avanzó y existen conflictos, Git se detiene sin sobreescribir tus archivos; informa los archivos en conflicto y mantiene el worktree intacto para resolución manual (`gz-ia session path <id>`) o abortar (`git merge --abort`).

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

---

### `gz-ia session prune`

Reconcilia y elimina sesiones, ramas de Git y worktrees huérfanos que hayan quedado desfasados por eliminaciones manuales o interrupciones.

```bash
gz-ia session prune [-d, --dir <directorio>]
```

---

### `gz-ia vault`

Gestiona secretos y variables de entorno centralizadas para perfiles y agentes con almacenamiento seguro local en `.harness/vault.json` (permisos estrictos `0600` e ignorado automáticamente por Git).

#### Subcomandos:

```bash
# Listar variables configuradas y recomendadas (con valores ofuscados)
gz-ia vault list [-d, --dir <directorio>]

# Guardar o actualizar una variable (solicita el secreto con entrada protegida)
gz-ia vault set <VARIABLE> [VALOR] [-v, --value <val>]

# Inspeccionar el estado y origen de una variable (ofuscada por defecto)
gz-ia vault get <VARIABLE> [--reveal]

# Eliminar una variable del vault
gz-ia vault delete <VARIABLE>

# Imprimir la ruta física del archivo vault.json
gz-ia vault path
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
- `-P, --profile <nombre>`: Perfil agéntico de tooling a activar.
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

Muestra la información de versión del binario compiled.

```bash
gz-ia version
```
