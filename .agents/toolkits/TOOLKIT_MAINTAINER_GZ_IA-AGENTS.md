# Directivas Maestras — toolkit-maintainer-gz-ia

Define las convenciones de ingeniería, directivas operativas y restricciones normativas para el mantenimiento y evolución del código fuente del arnés `gz-ia` (escrito en Go).

---

## 1. Filosofía de Ingeniería de gz-ia

El arnés `gz-ia` es una infraestructura liviana, modular, agnóstica de modelos de IA y descentralizada:

1. **Minimalismo Radical (De Menos a Más)**:
   - Prohibido introducir capas de abstracción prematuras o dependencias sobredimensionadas.
   - Construir exclusivamente lo necesario para resolver el caso de uso actual.
   - Utilizar herramientas estándar de Unix y librerías canónicas de Go (`cobra`, `bubbletea`, `lipgloss`, `huh`).

2. **Paridad Total CLI-First**:
   - Todo comportamiento debe ser operable e inspeccionable vía CLI mediante comandos y flags estándar.
   - La TUI interactiva es una capa de conveniencia visual montada sobre el core y la CLI; prohibido implementar capacidades en la TUI que no existan en la CLI.

3. **Todo Debe Tener Testing**:
   - Ninguna funcionalidad, refactorización o corrección se da por concluida sin pruebas unitarias e integradas (`go test -count=1 ./...`).

4. **Soberanía Humana de Git**:
   - Ningún agente tiene autorización para ejecutar `git commit` o `git push` en el repositorio principal sin consentimiento explícito del usuario.
   - El aislamiento de sesiones opera mediante Git Worktrees efímeros (`.harness/worktrees/<id>`).

5. **Portabilidad Absoluta**:
   - Cero rutas absolutas de desarrollo hardcodeadas (`/home/...`). Utilizar `os.UserHomeDir`, `filepath.Join` y `exec.LookPath`.
   - Compatibilidad multiplataforma (Linux, macOS, Windows).

---

## 2. Estructura Arquitectónica Interna en Go

- `cmd/gz-ia/`: Punto de entrada principal (`main.go`).
- `internal/clients/cli/`: Definición de comandos Cobra, flags y salida de terminal.
- `internal/clients/tui/`: Interfaz interactiva Bubble Tea / Huh.
- `internal/features/session/`: Ciclo de vida de sesiones, orquestación de drivers y aislamiento.
- `internal/features/tooling/`: Carga, resolución y scaffolding de toolkits y presets.
- `internal/features/workspace/`: Gestión de Git Worktrees, manifiestos de sesión y proyecciones.
- `internal/features/updater/`: Actualizaciones atómicas y verificación de checksums.
- `internal/features/metrics/`: Registro de consumo y telemetría local descentralizada.

---

## 3. Política de Cero Ruido (Zero-Noise Policy)

- Cero emojis en mensajes de error, logs de terminal, código fuente o documentación.
- Cero comentarios redundantes en código Go. Comentarios reservados únicamente para justificar decisiones no evidentes.
- Manejo idiomático y explícito de errores (`if err != nil { return fmt.Errorf(...) }`); prohibido silenciar errores con `_ =`.
