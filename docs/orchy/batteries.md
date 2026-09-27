# Baterías Incluidas: Plugins de Worktree

Orchy incorpora de fábrica el paquete de baterías `packages/orchy/batteries/worktree`, el cual provee herramientas para la manipulación controlada de worktrees:

1. **`worktree_read` (Activa por defecto):** Inspección segura de diffs y estados de ramas efímeras sin mutar el repositorio base. Expuesta a los agentes a través del servidor MCP.
2. **`worktree_get` (Deshabilitada por defecto):** Fusión e integración de modificaciones hacia el espacio de trabajo activo. Por diseño de seguridad (*human-in-the-loop*), la integración hacia el directorio principal es una acción humana (`gz-ia session get`) y no se expone a los agentes por defecto.

---

## Registro del Plugin `batteries.worktree`

El plugin se acopla directamente al ciclo de vida del kernel Orchy:

```go
import (
    "gz-ia/packages/orchy"
    "gz-ia/packages/orchy/batteries/worktree"
)

func main() {
    kernel := orchy.NewKernel()
    
    // Registrar el plugin de baterías de worktree (solo worktree_read por defecto)
    wtPlugin := worktree.NewWorktreePlugin("/path/al/proyecto")
    kernel.GetContext().Plugins.Register(wtPlugin)

    _ = kernel.Boot()
    // La herramienta 'worktree_read' queda registrada en el ToolRegistry.
    // 'worktree_get' solo se registraría si se pasa worktree.WithAllowAgentGet(true).
}
```

---

## <span id="worktree-read"></span>Herramienta: `worktree_read`

Permite a los agentes de IA (o subagentes revisores de código) examinar de forma no invasiva las modificaciones realizadas dentro del worktree de una sesión.

### Esquema MCP (Input Schema)

```json
{
  "type": "object",
  "properties": {
    "session_id": {
      "type": "string",
      "description": "Identificador único de la sesión de gz-ia."
    },
    "stat_only": {
      "type": "boolean",
      "description": "Si es true, muestra únicamente el resumen estadístico de archivos modificados."
    }
  },
  "required": ["session_id"]
}
```

### Formato de Respuesta (Output)

```json
{
  "session_id": "3f9a12c8",
  "diff": "diff --git a/main.go b/main.go\nindex 1234..5678 100644\n--- a/main.go\n+++ b/main.go\n@@ -10,2 +10,4 @@\n+func HealthCheck() string {\n+    return \"OK\"\n+}\n",
  "stat_only": false
}
```

---

## <span id="worktree-get"></span>Herramienta: `worktree_get`

Permite integrar las modificaciones validadas del worktree hacia la rama de trabajo activa.

### Esquema MCP (Input Schema)

```json
{
  "type": "object",
  "properties": {
    "session_id": {
      "type": "string",
      "description": "Identificador único de la sesión de gz-ia."
    },
    "no_commit": {
      "type": "boolean",
      "description": "Deja los cambios preparados en el stage del repo principal sin comitear automáticamente."
    },
    "squash": {
      "type": "boolean",
      "description": "Condensa los cambios en un único commit sin conservar el historial intermedio."
    }
  },
  "required": ["session_id"]
}
```

### Formato de Respuesta (Output)

```json
{
  "session_id": "3f9a12c8",
  "success": true,
  "message": "Cambios integrados exitosamente (squash merge)",
  "files": [
    "main.go",
    "internal/service.go"
  ]
}
```

---

## Casos de Uso Agénticos

1. **Revisión por Pares entre Agentes:** Un agente implementador escribe código en una sesión autónoma en un worktree. Un subagente revisor invoca `worktree_read` para analizar el diff y ejecutar linters antes de aprobar la integración.
2. **Promoción de Código:** Tras la aprobación del revisor, el agente orquestador llama a `worktree_get` con `no_commit: true`, permitiendo al desarrollador humano dar la última mirada en staging.
