# Ciclo de Vida y Persistencia de Sesiones

Cada interacción de un agente de inteligencia artificial en `gz-ia` se modela formalmente como una **Sesión**, gestionada por `internal/features/session`.

---

## Máquina de Estados de una Sesión

El ciclo de vida de una sesión sigue una máquina de estados determinista y libre de condiciones de carrera:

```mermaid
stateDiagram-v2
    [*] --> RUNNING: Lanzamiento gz-ia chat
    RUNNING --> COMPLETED: Agente finaliza con éxito
    RUNNING --> FAILED: Error de ejecución
    RUNNING --> KILLED: Interrupción forzada o señal
    RUNNING --> RUNNING: Reconexión TTY
    COMPLETED --> RUNNING: Reanudación de sesión
    FAILED --> RUNNING: Reanudación de sesión
    KILLED --> RUNNING: Reanudación de sesión
    COMPLETED --> [*]: Eliminación de sesión
    FAILED --> [*]: Eliminación de sesión
    KILLED --> [*]: Eliminación de sesión
```

### Definición de Estados

| Estado | Significado | Condiciones de Transición |
| :--- | :--- | :--- |
| `RUNNING` | El proceso del agente está en ejecución activa o el worktree está reservado. | Estado inicial tras invocar `gz-ia chat` o reanudar con `session resume`. |
| `COMPLETED` | La sesión terminó de forma regular con código de salida `0`. | El agente finalizó la tarea o el usuario cerró el chat normalmente. |
| `FAILED` | El agente finalizó con un código de error distinto de cero o abortó abruptamente. | Error interno del modelo, fallo de compilación o excepción no controlada. |
| `KILLED` | El proceso de la sesión fue terminado forzosamente por el usuario. | Invocación explícita de `gz-ia session kill <id>` o señal `SIGKILL`. |

---

## Estructura del Archivo de Registro (`SessionRecord`)

Toda la metadata de la sesión se almacena de forma persistente y atómica en formato JSON dentro de `.harness/sessions/<id>.json`:

```json
{
  "id": "3f9a12c8",
  "provider": "agy",
  "permission": "supervised",
  "status": "COMPLETED",
  "worktree_dir": "/home/user/project/.harness/worktrees/3f9a12c8",
  "is_worktree": true,
  "git_branch": "harness/3f9a12c8",
  "base_dir": "/home/user/project",
  "pid": 48120,
  "exit_code": 0,
  "prompt": "Implementar endpoints REST para autenticación",
  "created_at": "2026-09-24T18:00:00Z",
  "updated_at": "2026-09-24T18:32:10Z",
  "completed_at": "2026-09-24T18:32:10Z"
}
```

### Escritura Atómica

Para evitar corrupción de datos en caso de caídas del proceso o interrupciones del sistema, `gz-ia` escribe los registros primero en un archivo temporal (`.harness/sessions/<id>.json.tmp`) y luego realiza una sustitución atómica (`os.Rename`), manteniendo la consistencia en disco.

---

## Operaciones de Gestión de Sesiones

`gz-ia` provee comandos para administrar el ciclo de vida de las sesiones:

### Listado (`session list`)
Muestra una tabla con el identificador, agente, estado, rama y tiempo transcurrido de todas las sesiones registradas:

```bash
gz-ia session list
```

### Inspección (`session show`)
Despliega todos los campos del `SessionRecord`, incluyendo rutas completas, códigos de salida y métricas:

```bash
gz-ia session show 3f9a12c8
```

### Terminación de Procesos (`session kill`)
Envía una señal `SIGTERM` al grupo de procesos de la sesión y, si no responde tras el tiempo de espera, escala a `SIGKILL` (en Windows utiliza `taskkill /F /T`), actualizando el estado de la sesión a `KILLED`:

```bash
gz-ia session kill 3f9a12c8
```

### Reanudación de Sesiones (`session resume`)
Reconecta la terminal con el agente en el espacio de trabajo original. Detecta el proveedor registrado e inyecta la bandera correspondiente (`--continue` para `agy`/`opencode`, `--resume` para `claude`/`pi-agent`):

```bash
gz-ia session resume 3f9a12c8
# Alias abreviado:
gz-ia session continue 3f9a12c8
```

### Obtención de Ruta para Scripts (`session path`)
Imprime en `stdout` la ruta absoluta del directorio de trabajo de la sesión sin texto adicional, ideal para integrarse en scripts de shell:

```bash
cd $(gz-ia session path 3f9a12c8)
```
