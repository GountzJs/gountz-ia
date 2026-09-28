# Interfaz TUI Interactiva

`gz-ia` incorpora una interfaz de usuario en terminal (TUI) moderna, visual y accesible, construida con las librerías líderes del ecosistema Go: **Bubble Tea**, **Lipgloss** y **Huh** (`internal/clients/tui`).

---

## Cómo Iniciar la TUI

Puedes acceder a la interfaz interactiva simplemente ejecutando:

```bash
gz-ia
# o equivalentemente:
gz-ia start
```

---

## Anatomía de la Pantalla Principal

Al abrir `gz-ia`, la TUI presenta el banner de telemetría Fastfetch con arte Braille vectorial y el menú principal de navegación:

```text
╭────────────────────────────────────────────────────────────────────────────────────────╮
│ ⠀⠀⣀⣀⣀⣠⠤⠤⠤⠖⠒⠒⠊⠉⠉⠓⢄⡀⠀⠀⠀⠀⠀⠀   developer@gz-ia                                │
│ ⡞⠫⣅⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠉⠲⣄⠀⠀⠀⠀   ────────────────────────────────────────        │
│ ⡇⠀⠈⠓⢦⡀⠀⠀⠀⠀⢀⣀⣀⣀⣤⣤⣤⣶⣶⡒⠛⡆⠀⠀   >_ Harness     ❯ gz-ia (core)                   │
│ ⡇⠀⠀⠀⠀⠙⡷⠚⣻⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⡇⠀⠀   ✦ Versión      ❯ v0.4.0                         │
│ ⡇⠻⣆⠀⠀⠀⡇⣼⣿⣿⣿⠿⠿⢿⡿⠟⢉⣉⡁⠉⠉⡇⠀⠀   ⌥ Workspace    ❯ gz-ia                          │
│ ⡇⢀⣽⠇⠀⠀⡁⠉⠁⡤⣴⣶⣶⠀⣠⡀⠻⠿⠿⠏⢠⣇⠔⠀   ⚙ Arquitectura ❯ Desacoplada / Modular          │
│ ⡇⠛⢁⡀⠀⠀⡇⣿⣆⠐⠚⠛⠉⣠⣀⣠⣤⢰⣆⠀⠀⠈⠀⡀   📦 Runtime     ❯ go1.23.1                       │
│ ⢇⠀⠀⠙⠷⠀⡇⠸⣿⣿⣿⣿⣄⡈⣥⣤⣠⣾⣇⢓⣄⠀⠘⠃   🩺 Modo        ❯ Standalone / TUI               │
│ ⠀⠉⠢⣀⠀⠀⡇⠀⠙⢿⣿⣿⣿⣿⣮⣷⡿⠿⠛⡠⠄⠀⠀⠀   ✓ Estado       ❯ Listo (✓)                      │
│ ⠀⠀⠀⠀⠑⠤⣇⣀⣀⡤⠤⠭⠭⠝⠒⠒⠒⠊⠉⠀⠀⠀⠀⠀                                                    │
│       GountzJs          ● ● ● ● ● ● ● ●                                │
╰────────────────────────────────────────────────────────────────────────────────────────╯

  🤖  gz-ia — Base Agéntica
  ❯ Selecciona una opción y presiona Enter:
    • 🚀  Iniciar nuevo chat con agente
    • ⌥   Sesiones activas e historial
    • 🔄  Actualizar gz-ia (update)
    • ✕   Cerrar / Salir
```

1. **Banner Fastfetch con Arte Braille:** Representa fielmente el avatar de la marca GountzJs mediante caracteres Braille vectoriales en degradado azulino, complementado con información del entorno (usuario, workspace, runtime Go y estado de salud).
2. **Menú de Selección Rápida:** Navegación fluida por teclado para gestionar el ciclo de vida completo de tus agentes.

---

## Flujos Interactivos

### 1. Iniciar Nuevo Chat con un Agente

El asistente de lanzamiento opera en dos pasos guiados con validación activa:

1. **Selección de Motor de IA:**
   - Evalúa en vivo la presencia de los binarios en tu sistema.
   - Marca cada agente con `[✓]` (listo para usar) o `[✗]` (no instalado).
   - Preselecciona automáticamente el primer agente disponible.
   - Si seleccionas un agente no disponible en `$PATH`, la TUI previene el fallo y despliega de inmediato el comando de instalación sugerido (`InstallHint`).
2. **Selección del Nivel de Permisos:**
   - Permite escoger entre:
     - `Solo lectura`: Modo planificación o consulta, sin alterar archivos.
     - `Con autorización`: Confirmación paso a paso interactiva `[y/N]`.
     - `Autónomo`: Auto-aprobación de acciones dentro del Git worktree aislado.
   - Cada pantalla incluye la opción `[←] Volver al menú principal` para regresar a la vista anterior.

### 2. Exploración de Sesiones e Historial

Permite gestionar los registros locales de `.harness/sessions/`:
- **Ver Detalles:** Muestra el ID de sesión, estado (`RUNNING`, `COMPLETED`, `FAILED`, `KILLED`), rama de git asociada, ruta del worktree y métricas de ejecución.
- **Inspeccionar Worktree (`read`):** Visor integrado de diffs con coloreado de sintaxis (altas en verde, bajas en rojo, hunks en azul).
- **Traer Cambios (`get`):** Diálogo interactivo de confirmación para incorporar cambios validados al workspace actual como modificaciones no preparadas (*unstaged*).
- **Reanudar Sesión (`resume`):** Reconecta la terminal con el agente original usando `--continue` o `--resume` en su worktree correspondiente.
- **Detener Proceso (`kill`):** Envía señales (`SIGTERM` seguido de `SIGKILL` si es necesario en Unix, o `taskkill /F /T` en Windows) para finalizar agentes en segundo plano.
- **Eliminar Registro (`delete`):** Limpia la metadata de la sesión y destruye el worktree y la rama de Git asociada de forma segura.

### 3. Actualizador Integrado

Permite comprobar la existencia de nuevas versiones publicadas en GitHub Releases:
- Consulta las versiones en segundo plano sin bloquear la terminal.
- Si hay una actualización disponible, muestra los detalles y solicita confirmación con un cuadro de diálogo `huh.NewConfirm()`.
- Si el binario ya está al día, muestra la notificación correspondiente en pantalla.

---

## Accesibilidad y Control de Terminal

### 1. Opciones de Retorno y Escape
- **Opciones de Retorno Explícitas:** Cada submenú, formulario o vista de detalle incluye una opción `[←] Volver al menú principal` o `Volver`.
- **Comportamiento de la Tecla `Esc`:** Al pulsar `Esc` en un submenú o diálogo modal, el runtime captura `huh.ErrUserAborted` y regresa al nivel anterior sin perder el estado del sistema. En el menú principal, `Esc` finaliza la aplicación restaurando la terminal.
- **Salida con `Ctrl+C`:** Termina la sesión restaurando los modos del terminal (cursor visible, raw mode deshabilitado).

### 2. Modo Accesible para Screen Readers y TTY Redirigidas
- Al detectar entornos sin soporte TTY completo (como pipes de testing o terminales accesibles para lectores de pantalla), `gz-ia` conmuta a `WithAccessible(true)`.
- En este modo se omiten secuencias ANSI de control de cursor y se presentan opciones lineales con selección numérica.

### 3. Soporte UTF-8
- La TUI es compatible con terminales con fuentes Nerd Font y emuladores estándar con soporte UTF-8.

---

## Atajos de Teclado

| Tecla / Combinación | Acción |
| :--- | :--- |
| `↑` / `k` | Mover la selección hacia arriba |
| `↓` / `j` | Mover la selección hacia abajo |
| `Enter` | Confirmar y ejecutar la opción seleccionada |
| `Esc` / `q` | Volver al menú anterior o cancelar la operación |
| `Ctrl+C` | Salir limpiamente de la TUI restaurando la terminal |

---

## Filosofía CLI-First: Todo lo de la TUI está en la CLI

Recuerda que la TUI es únicamente una interfaz visual. **Absolutamente todas** las acciones realizables en la TUI cuentan con su equivalente exacto en la línea de comandos:

- Iniciar chat $\rightarrow$ `gz-ia chat -p <driver> -m <perm>`
- Listar sesiones $\rightarrow$ `gz-ia session list`
- Inspeccionar contexto $\rightarrow$ `gz-ia session context <id> [-j]`
- Inspeccionar worktree $\rightarrow$ `gz-ia session read <id> [--stat]`
- Traer cambios $\rightarrow$ `gz-ia session get <id>`
- Reanudar sesión $\rightarrow$ `gz-ia session resume <id>`
- Matar proceso $\rightarrow$ `gz-ia session kill <id>`
- Actualizar binario $\rightarrow$ `gz-ia update`
