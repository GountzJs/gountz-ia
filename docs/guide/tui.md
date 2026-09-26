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
│ ⡇⠀⠀⠀⠀⠙⡷⠚⣻⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⡇⠀⠀   ✦ Versión      ❯ v0.0.1                         │
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
   - Cada pantalla incluye la opción explícita `[←] Volver al menú principal` para garantizar libertad total de navegación.

### 2. Exploración de Sesiones e Historial

Permite gestionar los registros locales de `.harness/sessions/`:
- **Ver Detalles:** Muestra el ID de sesión, estado (`RUNNING`, `COMPLETED`, `FAILED`, `KILLED`), rama de git asociada, ruta del worktree y métricas de ejecución.
- **Inspeccionar Worktree (`read`):** Visor integrado de diffs con coloreado de sintaxis (altas en verde, bajas en rojo, hunks en azul).
- **Traer Cambios (`get`):** Diálogo interactivo de confirmación para incorporar cambios validados al workspace actual mediante squash o commit regular.
- **Reanudar Sesión (`resume`):** Reconecta la terminal con el agente original usando `--continue` o `--resume` en su worktree correspondiente.
- **Detener Proceso (`kill`):** Envía señales controladas (`SIGTERM` seguido de `SIGKILL` si es necesario) para finalizar agentes en segundo plano.
- **Eliminar Registro (`delete`):** Limpia la metadata de la sesión y destruye el worktree y la rama de Git asociada de forma segura.

### 3. Actualizador Integrado

Permite comprobar la existencia de nuevas versiones publicadas en Nexus o Forgejo:
- Consulta los endpoints en segundo plano sin congelar la terminal.
- Si hay una actualización disponible, muestra los detalles y solicita confirmación con un cuadro de diálogo `huh.NewConfirm()`.
- Si el binario ya está al día, informa amigablemente con un panel de estilo Lipgloss.

---

## Experiencia de Accesibilidad y Navegación Segura

Uno de los principios de diseño de `gz-ia` es la **accesibilidad universal** y la garantía de que **ningún usuario quede atrapado**:

### 1. Garantía Anti-Atrapamiento (Escape Hatch)
- **Opciones de Retorno Explícitas:** Cada submenú, formulario o vista de detalle incluye invariablemente una opción explícita como `[←] Volver al menú principal` o `Volver`. No depende exclusivamente de recordar atajos de teclado.
- **Comportamiento Universal de la Tecla `Esc`:** Al pulsar `Esc` en cualquier submenú o diálogo modal, el runtime de `huh` captura `huh.ErrUserAborted` y redirige el flujo de regreso al nivel anterior sin abortar la aplicación ni perder el estado del sistema. Pulsar `Esc` en el menú principal finaliza la aplicación de forma limpia y elegante.
- **Salida Segura con `Ctrl+C`:** Termina la sesión restaurando los modos del terminal (cursor visible, raw mode deshabilitado), garantizando que tu shell nunca quede en un estado corrupto.

### 2. Modo Accesible para Screen Readers y TTY Redirigidas
- Al detectar entornos sin soporte TTY completo (como pipes de testing o terminales accesibles para lectores de pantalla), `gz-ia` conmuta a `WithAccessible(true)`.
- En este modo se eliminan los caracteres ANSI de control de cursor y se presentan preguntas lineales estándar con selección numérica legible por lectores de pantalla.

### 3. Degradación Elegante UTF-8
- La TUI está diseñada para funcionar a la perfección tanto en terminales con fuentes completas Nerd Font como en emuladores estándar con soporte UTF-8 básico.

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
- Inspeccionar worktree $\rightarrow$ `gz-ia session read <id> [--stat]`
- Traer cambios $\rightarrow$ `gz-ia session get <id> [--no-commit]`
- Reanudar sesión $\rightarrow$ `gz-ia session resume <id>`
- Matar proceso $\rightarrow$ `gz-ia session kill <id>`
- Actualizar binario $\rightarrow$ `gz-ia update`
