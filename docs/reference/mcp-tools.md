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

Batería interna de Orchy para traer las modificaciones desde el worktree hacia el directorio de trabajo activo. **No se expone a los agentes por defecto**, requiriendo `worktree.WithAllowAgentGet(true)` para ser registrada en el MCP Server. Trae las modificaciones como cambios no preparados (*unstaged*).

### Especificación JSON-RPC (`tools/list`)

```json
{
  "name": "worktree_get",
  "description": "Trae e integra las modificaciones producidas en el worktree de sesión hacia el directorio de trabajo activo (unstaged).",
  "inputSchema": {
    "type": "object",
    "properties": {
      "session_id": {
        "type": "string",
        "description": "Identificador único de la sesión de gz-ia."
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
      "session_id": "3f9a12c8"
    }
  }
}
```

---

## 3. Batería de Observabilidad (`session_log`)

Permite a los agentes emitir eventos de trazabilidad y registrar hitos de su ciclo de trabajo (`READ`, `PENDING`, `FINISH`) directamente en el archivo `.events.jsonl` de la sesión activa desde cualquier cliente MCP.

### `session_log`

#### Especificación JSON-RPC (`tools/list`)

```json
{
  "name": "session_log",
  "description": "Registra un evento estructurado de observabilidad en el archivo de log (.events.jsonl) de la sesión activa.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "action": {
        "type": "string",
        "description": "Descripción breve de la acción realizada."
      },
      "stage": {
        "type": "string",
        "enum": ["READ", "PENDING", "FINISH"],
        "description": "Etapa del ciclo de trabajo."
      },
      "status": {
        "type": "string",
        "enum": ["OK", "FAILED"],
        "description": "Resultado de la etapa (requerido para FINISH)."
      },
      "role": {
        "type": "string",
        "description": "Rol del subagente o agente."
      },
      "agent": {
        "type": "string",
        "description": "Identificador único del subagente o agente."
      },
      "duration": {
        "type": "integer",
        "description": "Duración de la ejecución en milisegundos."
      },
      "error": {
        "type": "string",
        "description": "Detalle del error si el estado fue FAILED."
      },
      "session_id": {
        "type": "string",
        "description": "Identificador de sesión (opcional si existe sesión por defecto)."
      }
    },
    "required": ["action"]
  }
}
```

#### Invocación (`tools/call`)

```json
{
  "jsonrpc": "2.0",
  "id": 12,
  "method": "tools/call",
  "params": {
    "name": "session_log",
    "arguments": {
      "action": "Ejecutando suite de pruebas unitarias",
      "stage": "PENDING",
      "agent": "sub-tester",
      "role": "QA"
    }
  }
}
```

---

## 4. Batería de Memoria (`memory`)

Expone capacidades de persistencia, búsqueda semántica (BM25) y consolidación de conocimientos y decisiones arquitectónicas para los agentes.

### `memory_save`

Guarda una entrada de memoria de contexto, regla de negocio o decisión.

#### Especificación JSON-RPC (`tools/list`)

```json
{
  "name": "memory_save",
  "description": "Guarda un registro de decisión, regla o conocimiento en la memoria local de gz-ia.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "title": {
        "type": "string",
        "description": "Título descriptivo del conocimiento o decisión."
      },
      "content": {
        "type": "string",
        "description": "Contenido detallado, código o especificación."
      },
      "category": {
        "type": "string",
        "description": "Categoría (ej. 'decision', 'rule', 'architecture', 'bugfix')."
      },
      "tags": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Etiquetas para facilitar filtrado."
      },
      "session_id": {
        "type": "string",
        "description": "Identificador de la sesión (opcional; si se omite se guarda en el proyecto)."
      }
    },
    "required": ["title", "content"]
  }
}
```

#### Invocación (`tools/call`)

```json
{
  "jsonrpc": "2.0",
  "id": 13,
  "method": "tools/call",
  "params": {
    "name": "memory_save",
    "arguments": {
      "title": "Decisión de Soberanía Estricta de Git",
      "content": "session.Service.Get trae los cambios como unstaged sin git merge commits automáticos.",
      "category": "architecture",
      "tags": ["git", "workspace", "session"]
    }
  }
}
```

### `memory_search`

Busca memorias ordenadas por el algoritmo Okapi BM25 por relevancia semántica.

#### Especificación JSON-RPC (`tools/list`)

```json
{
  "name": "memory_search",
  "description": "Busca decisiones y contexto usando el motor BM25 por relevancia semántica.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "Consulta de búsqueda o términos clave."
      },
      "session_id": {
        "type": "string",
        "description": "ID de sesión para incluir su memoria local (opcional)."
      },
      "global_only": {
        "type": "boolean",
        "description": "Buscar exclusivamente en la memoria del proyecto."
      },
      "limit": {
        "type": "integer",
        "description": "Límite de resultados (por defecto 10)."
      }
    },
    "required": ["query"]
  }
}
```

#### Invocación (`tools/call`)

```json
{
  "jsonrpc": "2.0",
  "id": 14,
  "method": "tools/call",
  "params": {
    "name": "memory_search",
    "arguments": {
      "query": "soberania git unstaged",
      "limit": 5
    }
  }
}
```

### `memory_list`

Lista los registros de memoria guardados en el ámbito especificado.

#### Especificación JSON-RPC (`tools/list`)

```json
{
  "name": "memory_list",
  "description": "Lista todos los registros de memoria guardados en la sesión o el proyecto.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "session_id": {
        "type": "string",
        "description": "ID de sesión (opcional)."
      },
      "global_only": {
        "type": "boolean",
        "description": "Listar exclusivamente memorias del proyecto."
      }
    }
  }
}
```

### `memory_consolidate`

Promueve y consolida las entradas de memoria de una sesión hacia el almacén global del proyecto.

#### Especificación JSON-RPC (`tools/list`)

```json
{
  "name": "memory_consolidate",
  "description": "Promueve y consolida las memorias de una sesión hacia el proyecto global.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "session_id": {
        "type": "string",
        "description": "Identificador de la sesión a consolidar."
      }
    },
    "required": ["session_id"]
  }
}
```

#### Invocación (`tools/call`)

```json
{
  "jsonrpc": "2.0",
  "id": 15,
  "method": "tools/call",
  "params": {
    "name": "memory_consolidate",
    "arguments": {
      "session_id": "3f9a12c8"
    }
  }
}
```
