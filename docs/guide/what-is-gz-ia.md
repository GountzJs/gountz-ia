# ¿Qué es gz-ia?

**`gz-ia`** es un *Harness* (arnés de orquestación y contención) de terminal para agentes de inteligencia artificial de desarrollo de software (`agy`, `claude code`, `opencode`, `pi-agent`).

Su propósito es desacoplar el entorno de trabajo del desarrollador respecto a las modificaciones que realiza el agente, ofreciendo un flujo unificado de ejecución, revisión e integración en Git, junto con observabilidad estructurada local.

---

## ¿Por qué un Harness?

Al interactuar con agentes de terminal que editan código de forma autónoma o supervisada surgen varios problemas prácticos:

1. **Colisión en el espacio de trabajo activo:** Si el agente modifica archivos mientras trabajas en tu editor, ensucia tu `git status`, altera tu rama activa o rompe la compilación en caliente (*hot-reload*).
2. **Incompatibilidad de sintaxis entre herramientas:** Cada CLI (`agy`, `claude`, `opencode`) utiliza sus propias banderas para permisos, prompts iniciales o reanudación de sesiones.
3. **Flujos de integración fragmentados:** Revisar y fusionar los cambios de un agente suele exigir comandos manuales de Git, creación de ramas temporales o inspección de diffs poco estructurados.
4. **Falta de observabilidad local unificada:** Resulta difícil auditar qué pasos completó el agente, en qué etapa falló o consultar métricas de ejecución sin depender de plataformas en la nube.

`gz-ia` aborda estos puntos mediante una capa homogénea basada en **Drivers de agentes**, desacoplamiento mediante **Git Worktrees** locales y un **Microkernel interno (Orchy)** que provee herramientas mediante el protocolo MCP (Model Context Protocol).

---

## Qué Aísla y Qué No Aísla el Harness

Es fundamental comprender con exactitud las fronteras técnicas de aislamiento que provee `gz-ia`:

### Lo que sí aísla: El árbol de trabajo (Working Tree)
- **Archivos desacoplados:** Cada sesión se ejecuta en `.harness/worktrees/<id>` sobre una rama dedicada (`harness/<id>`).
- **Tu editor no se altera:** El agente puede crear, editar o eliminar archivos en su worktree sin alterar los archivos abiertos en tu editor ni interferir con tu trabajo en paralelo.
- **`git status` limpio en tu rama:** Tu rama base no registra cambios mientras el agente trabaja.

### Lo que NO es: No es un sandbox de sistema ni de kernel
- **Repositorio Git compartido:** Todos los worktrees comparten el mismo directorio `.git`, refs y almacén de objetos.
- **Acceso a shell y comandos de sistema:** Si configuras un agente en modo autónomo con herramientas de ejecución en terminal, dicho agente corre con los privilegios de tu usuario del sistema operativo. Podría ejecutar comandos de shell o comandos de Git destructivos (`git branch -D`, `git push --force`).
- **Aislamiento de sistema:** Para contener red, procesos y sistema de archivos a nivel de kernel, se requiere ejecutar el entorno dentro de contenedores (Docker / DevContainers) o máquinas virtuales.

---

## Principios de Diseño

El diseño de `gz-ia` sigue pautas de ingeniería directas y prácticas:

### 1. Minimalismo y herramientas estándar
Sin demonios en segundo plano ni bases de datos externas. Emplea utilidades estándar (`git`, señales POSIX) y librerías en Go (`spf13/cobra`, `charmbracelet/lipgloss`, `charmbracelet/huh`).

### 2. Motor de IA agnóstico (Cero roles inventados)
`gz-ia` no inventa personalidades artificiales (`developer`, `architect`, `qa`). Orquesta binarios existentes mediante la interfaz `Driver` y normaliza las intenciones de ejecución:
- `readonly`: Modo de análisis o planificación sin modificaciones.
- `supervised`: Confirmación interactiva humana antes de ejecutar herramientas o comandos.
- `autonomous`: Auto-aprobación dentro del worktree de la sesión.

### 3. Integración en Git con control humano
El agente sólo tiene acceso a herramientas de inspección (`worktree_read`). La decisión y el acto de integrar el código generado hacia la rama base del repositorio (`gz-ia session get`) recaen exclusivamente en el desarrollador.

### 4. Paridad CLI-First
Toda acción disponible en la interfaz interactiva de terminal (TUI) es 100% ejecutable y automatizable desde la línea de comandos de forma idempotente.

### 5. Persistencia local y descentralizada
La metadata de las sesiones (`.harness/sessions/<id>.json`) y el log estructurado de eventos (`.events.jsonl`) se almacenan localmente dentro del repositorio del proyecto.

### 6. Microkernel interno Orchy
Microkernel modular en `packages/orchy` que implementa un servidor MCP JSON-RPC 2.0 por Stdio, Circuit Breaker (`HEALTHY`, `DEGRADED`, `DEAD`) y un bus de eventos interno para comunicar componentes.

---

## Diferencial Real: Con y Sin Harness

| Aspecto | Ejecución Directa de Agentes | Con `gz-ia` Harness |
| :--- | :--- | :--- |
| **Espacio de Trabajo** | Modifica los archivos en tu directorio actual | Opera en worktree desacoplado (`.harness/worktrees/<id>`) |
| **Flujo de Integración** | Commits manuales o stashes en tu rama | Flujo uniforme: inspección (`read`), diff (`diff`) y merge (`get`) |
| **Concurrencia** | Difícil correr múltiples agentes en paralelo | Múltiples sesiones simultáneas en ramas independientes |
| **Control de Privilegios** | Banderas y sintaxis dispar por cada agente | Interfaz normalizada (`readonly`, `supervised`, `autonomous`) |
| **Observabilidad** | Salida dispersa de consola | Registro estructurado de etapas (`READ` → `PENDING` → `FINISH`) |
| **Herramientas MCP** | Servidores externos requeridos por separado | Servidor MCP nativo Stdio embebido para inspección |

