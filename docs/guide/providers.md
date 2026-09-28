# Agentes Soportados (Multi-Driver)

`gz-ia` implementa el patrón de diseño **Driver** (`internal/features/session/driver.go`) para interactuar con distintas CLIs de agentes de IA de terminal (`agy`, `claude`, `opencode`, `pi-agent`).

El arnés detecta los binarios en tu `PATH` (`exec.LookPath`) y traduce las intenciones de ejecución (`readonly`, `supervised`, `autonomous`) en las banderas nativas que cada herramienta soporta.

---

## Matriz de Banderas y Alcance por Driver

::: warning Alcance de los Permisos
Los niveles `readonly`, `supervised` y `autonomous` corresponden a un mapeo hacia las banderas de la CLI subyacente. El comportamiento efectivo depende de cómo cada agente procesa dichas opciones.
:::

| Driver (`ID`) | Binario | Nivel de Permiso | Banderas Pasadas al Proceso | Alcance Operativo | Limitaciones |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`agy`** | `agy` | `readonly`<br>`supervised`<br>`autonomous` | `--mode plan`<br>*(ninguna adicional)*<br>`--dangerously-skip-permissions` | En `readonly`, Antigravity no edita archivos.<br>En `supervised`, solicita confirmación interactiva `[y/N]`.<br>En `autonomous`, auto-aprueba herramientas. | En `autonomous`, el agente tiene acceso a comandos de shell en el host si sus herramientas lo permiten. |
| **`claude`** | `claude` | `readonly`<br>`supervised`<br>`autonomous` | *(ninguna adicional)*<br>*(ninguna adicional)*<br>`--dangerously-skip-permissions` | En `supervised`, Claude Code solicita confirmación para comandos bash y mutaciones.<br>En `autonomous`, omite confirmaciones. | `claude` **no cuenta con un modo `--mode plan` nativo**. En `readonly`, el agente corre en modo interactivo sin banderas de bypass; si el modelo decide invocar herramientas de mutación, el modo solo lectura es declarativo a menos que el usuario rechace el prompt interactivo. |
| **`opencode`** | `opencode` | `readonly`<br>`supervised`<br>`autonomous` | *(ninguna adicional)*<br>*(ninguna adicional)*<br>`--dangerously-skip-permissions` | En `supervised`, pide confirmación antes de aplicar parches.<br>En `autonomous`, ejecuta sin interrupciones. | No dispone de sandbox de solo lectura garantizado por CLI; requiere supervisión humana interactiva en terminal. |
| **`pi-agent`** | `pi-agent` | `readonly`<br>`supervised`<br>`autonomous` | *(ninguna adicional)*<br>*(ninguna adicional)*<br>`--dangerously-skip-permissions` | En `supervised`, interacción estándar en terminal.<br>En `autonomous`, auto-aprobación de acciones. | Idem: la contención de archivos depende del worktree de Git, no de restricciones del binario de Pi Agent. |

---

## Banderas de Inyección de Prompt y Reanudación

Cada driver traduce también las opciones de prompt inicial y reanudación de sesiones:

| Driver (`ID`) | Reanudación (`gz-ia session resume`) | Prompt Inicial (`gz-ia chat -i "..."`) |
| :--- | :--- | :--- |
| `agy` | `--continue` | `-i <prompt>` |
| `claude` | `--resume` | `-p <prompt>` |
| `opencode` | `--continue` | `<prompt>` (argumento posicional) |
| `pi-agent` | `--resume` | `-p <prompt>` |

---

## Detalle e Instalación por Agente

### Google Antigravity (`agy`)

Agente de codificación de Google DeepMind / Google Antigravity.

- **Binario:** `agy`
- **Soporte de plan nativo:** `readonly` pasa `--mode plan`, impidiendo modificaciones directas en el sistema de archivos durante la planificación.
- **Transcripts locales:** `gz-ia session metrics` procesa directamente los ficheros `transcript.jsonl` de Antigravity para desglosar tokens y herramientas de subagentes.

::: info Instalación
Consulta la documentación oficial de Google Antigravity CLI para tu entorno.
:::

---

### Anthropic Claude Code (`claude`)

Herramienta oficial de línea de comandos de Anthropic asistida por Claude 3.7 Sonnet.

- **Binario:** `claude`
- **Comportamiento en permisos:** `supervised` es el modo recomendado; solicita confirmación interactiva para herramientas que alteran archivos o ejecutan comandos en shell.
- **Modo autónomo:** Se habilita mediante `--dangerously-skip-permissions`.

::: tip Instalación
```bash
npm install -g @anthropic-ai/claude-code
```
:::

---

### OpenCode (`opencode`)

Agente de terminal de código abierto.

- **Binario:** `opencode`
- **Comportamiento:** Pasa el prompt inicial como argumento posicional directo.

::: tip Instalación
```bash
curl -fsSL https://opencode.ai/install | bash
```
:::

---

### Pi Agent (`pi-agent`)

Agente ligero de terminal.

- **Binario:** `pi-agent`
- **Comportamiento:** Emplea `-p` para prompts iniciales y `--resume` para reanudar sesiones.

::: info Instalación
Consulta las instrucciones de distribución de Pi Agent para tu sistema operativo.
:::

---

## Cómo Seleccionar el Agente

### Desde la Línea de Comandos (CLI)

```bash
# Iniciar chat con Claude Code
gz-ia chat --provider claude

# Iniciar chat con Google Antigravity en modo supervisado
gz-ia chat -p agy -m supervised

# Iniciar sesión con OpenCode en modo autónomo dentro del worktree
gz-ia chat -p opencode -m autonomous

# Si no se especifica, gz-ia selecciona el primer agente disponible en PATH
gz-ia chat
```

### Desde la TUI Interactiva

Al ejecutar `gz-ia start` (o simplemente `gz-ia`), el selector interactivo mostrará en tiempo real qué agentes están detectados en el sistema:

```text
? ¿Qué agente de terminal deseas utilizar?
  > [✓] Google Antigravity (agy)
    [✓] Claude Code (claude)
    [✗] OpenCode (opencode) - No instalado
    [✗] Pi Agent (pi-agent) - No instalado
    [←] Volver al menú principal
```
