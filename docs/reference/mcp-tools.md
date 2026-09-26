# Esquemas de Herramientas MCP

Referencia técnica de las especificaciones y esquemas JSON Schema utilizados por el servidor MCP nativo de Orchy.

::: warning Política de Seguridad y Control Humano (Human-in-the-Loop)
Por defecto, el servidor MCP expone a los agentes **únicamente `worktree_read`**. La integración de código hacia el directorio de trabajo activo (`gz-ia session get`) es una acción **exclusivamente humana**. La herramienta `worktree_get` permanece deshabilitada por defecto en el plugin de baterías para preservar el control del desarrollador y sólo puede habilitarse mediante configuración explícita (`WithAllowAgentGet(true)`).
:::

---

## 1. `worktree_read` *(Expuesta por defecto)*

Inspecciona el contenido y diff actual de un worktree de sesión de `gz-ia` sin alterar el workspace base.

### Especificación JSON-RPC (`tools/list`)

```json
{
  "name": "worktree_read",
  "description": "Inspecciona el contenido y diff actual de un worktree de sesión de gz-ia sin alterar el workspace base.",
  "inputSchema": {
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
}
```

### Invocación (`tools/call`)

```json
{
  "jsonrpc": "2.0",
  "id": 10,
  "method": "tools/call",
  "params": {
    "name": "worktree_read",
    "arguments": {
      "session_id": "3f9a12c8",
      "stat_only": false
    }
  }
}
```

---

## 2. `worktree_get` *(Deshabilitada por defecto — Human-in-the-Loop)*

Batería interna de Orchy para fusionar modificaciones desde el worktree hacia el directorio de trabajo activo. **No se expone a los agentes por defecto**, requiriendo `worktree.WithAllowAgentGet(true)` para ser registrada en el MCP Server.

### Especificación JSON-RPC (`tools/list`)

```json
{
  "name": "worktree_get",
  "description": "Trae e integra las modificaciones producidas en el worktree de sesión hacia el directorio de trabajo activo.",
  "inputSchema": {
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
}
```

### Invocación (`tools/call`)

```json
{
  "jsonrpc": "2.0",
  "id": 11,
  "method": "tools/call",
  "params": {
    "name": "worktree_get",
    "arguments": {
      "session_id": "3f9a12c8",
      "squash": true,
      "no_commit": false
    }
  }
}
```
