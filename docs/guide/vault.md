# Variables de Entorno y Secretos Locales (Vault)

Gountz IA (`gz-ia`) incluye un gestor local de variables de entorno y credenciales (`internal/features/vault`), diseñado para suministrar configuraciones a los agentes de IA de forma centralizada sin modificar las variables del shell global ni incluirlas en el historial de Git.

---

## 1. Seguridad y Almacenamiento

El Vault se administra mediante el archivo local `.harness/vault.json`:

- **Formato en Texto Plano:** Se almacena en formato JSON estándar sin cifrado simétrico en reposo con contraseña maestra. No introduzcas tokens bancarios ni credenciales de infraestructura crítica sin evaluar tu modelo de amenazas local.
- **Permisos Estrictos POSIX `0600`:** Únicamente tu usuario del sistema operativo tiene permisos de lectura y escritura (`rw-------`). Cualquier intento de lectura o modificación por otros usuarios locales del sistema operativo es bloqueado por los permisos del sistema de archivos.
- **Aislamiento en Git vía `.git/info/exclude`:** La carpeta `.harness/` se encuentra excluida automáticamente del seguimiento de Git mediante `.git/info/exclude` local, evitando que el archivo se incluya en el repositorio remoto o afecte el `git status`.
- **Escrituras Atómicas:** Toda operación (`set`, `delete`) escribe primero en un archivo temporal (`.vault.json.tmp`) antes de aplicar un reemplazo atómico con `os.Rename`, evitando archivos corruptos si se interrumpe el comando.

> [!WARNING] Modelo de Acceso del Agente en Sesión
> Ten en cuenta dos consideraciones sobre cómo el agente accede a los secretos:
> 1. **Inyección de variables de entorno:** Al iniciar una sesión, `gz-ia` inyecta las variables guardadas en el vault como variables de entorno del proceso hijo del agente. Cualquier comando ejecutado en la sesión (por ejemplo, `env` o `printenv`) tendrá visibilidad de estas claves.
> 2. **Ruta física accesible desde el worktree:** Desde el árbol de trabajo de la sesión (`.harness/worktrees/<id>`), el archivo `.harness/vault.json` se encuentra a dos niveles de distancia (`../../vault.json`). Un agente con herramientas de lectura de disco o permisos de terminal puede leer el archivo directamente.

---

## 2. Configuración de API Keys

> [!IMPORTANT] Claude Code y API Keys
> Si utilizas **Claude Code** con suscripción oficial Pro o Team (autenticado mediante `claude login`), **no almacenes `ANTHROPIC_API_KEY` en el vault**.
> 
> Cuando `ANTHROPIC_API_KEY` está presente en el entorno de ejecución, Claude Code prioriza esa clave y factura el consumo de tokens a la cuenta de la API de Anthropic, en lugar de utilizar los límites de tu suscripción.
> 
> En **OpenCode**, solo configura claves de API si tu modelo o proveedor específico lo requiere.

`gz-ia` no impone claves fijas para iniciar sesiones. Únicamente advertirá sobre variables no configuradas si los toolkits o presets que actives declaran explícitamente dependencias en su propiedad `env`.

---

## 3. Comandos CLI: `gz-ia vault`

El comando `gz-ia vault` (o `gz-ia vault list` por defecto) administra las variables del proyecto:

### Listar Variables y Estado de Configuración
```bash
gz-ia vault list [-d <directorio>]
```

**Salida de ejemplo:**
```text
Variables de Entorno y Secretos (Vault) — Gountz IA
Archivo: /home/usuario/proyecto/.harness/vault.json (Permisos 0600, ignorado por Git)

VARIABLE                  ESTADO        VALOR ENMASCARADO         RECOMENDADA PARA    
─────────────────────────────────────────────────────────────────────────────────────────────
DATABASE_URL              Vault [✓]     postgres://u:*******5432  toolkit-db
GITHUB_TOKEN              Sistema [$]   ghp_******************ab  herramientas CI/Git
STRIPE_KEY                Faltante      (no configurada)          toolkit-billing
```

- **Estados identificados:**
  - `Vault [✓]`: Almacenada y protegida en `.harness/vault.json`.
  - `Sistema [$]`: Detectada en las variables de entorno del sistema operativo (`$ENV`).
  - `Faltante`: Variable requerida por un toolkit o preset activo pero no encontrada.
- **Valores protegidos:** Todos los secretos se muestran ofuscados (`MaskSecret`), revelando únicamente el prefijo y sufijo mínimo necesario.

### Guardar o Actualizar una Variable
```bash
# Modo interactivo seguro (entrada enmascarada sin dejar rastro en el historial de shell)
gz-ia vault set DATABASE_URL

# Modo directo pasando el valor como argumento
gz-ia vault set DATABASE_URL "postgres://usuario:pass@localhost:5432/db"
# o mediante flag -v
gz-ia vault set DATABASE_URL -v "postgres://usuario:pass@localhost:5432/db"
```

> [!TIP] Prevención de fuga en `.bash_history`
> Si omites el valor al ejecutar `gz-ia vault set <CLAVE>`, el comando abre un formulario interactivo con `EchoModePassword`. El texto introducido no se imprime en pantalla ni queda registrado en el historial de comandos del shell.

### Consultar el Estado y Origen de una Variable
```bash
# Inspección segura con valor enmascarado
gz-ia vault get DATABASE_URL

# Revelar el valor completo en texto plano (útil para tuberías y scripts locales)
gz-ia vault get DATABASE_URL --reveal
```

### Eliminar una Variable
```bash
gz-ia vault delete DATABASE_URL
# Aliases disponibles:
gz-ia vault rm DATABASE_URL
gz-ia vault remove DATABASE_URL
```

### Obtener la Ruta Física del Almacén
```bash
gz-ia vault path
```
Imprime la ruta absoluta hacia `.harness/vault.json`, permitiendo su referencia en scripts o diagnósticos.

---

## 4. Detección de Variables al Iniciar Sesiones

Al iniciar una sesión de chat (`gz-ia chat` o desde la TUI), el arnés analiza los requerimientos declarados en los **toolkits y presets activos**:

1. **Inspección de Toolkits:**
   Si los toolkits seleccionados definen variables en su campo `env` (por ejemplo, `DATABASE_URL` o `API_SECRET`), `gz-ia` verifica si existen en el sistema o en el vault local.
2. **Aviso No Bloqueante:**
   Si detecta que una variable requerida no está configurada, despliega un aviso informativo en pantalla antes de iniciar la sesión:
   ```text
   Aviso de Entorno: Se detectaron variables no configuradas en el entorno ni en el vault:
      • DATABASE_URL
      Puedes configurarlas en el vault seguro con: gz-ia vault set DATABASE_URL
   ```

---

## 5. Inyección en Procesos de IA

Cuando el agente (`agy`, `claude`, `opencode`, `pi-agent`) es ejecutado:
- `gz-ia` toma las variables de entorno actuales del sistema operativo y les superpone las almacenadas en `.harness/vault.json`.
- Las variables del vault tienen precedencia y se inyectan en el entorno del proceso hijo del agente sin modificar las variables globales del shell del usuario.
