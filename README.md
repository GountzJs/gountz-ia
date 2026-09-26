<div align="center">

# Gountz IA (`gz-ia`)

**Universal Terminal AI Harness & Git Worktree Sandbox**

[![Release](https://img.shields.io/github/v/release/GountzJs/gountz-ia?color=38bdf8&logo=github&style=flat-square)](https://github.com/GountzJs/gountz-ia/releases)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![Platforms](https://img.shields.io/badge/platforms-linux%20%7C%20windows%20(amd64)-22c55e?style=flat-square)](https://github.com/GountzJs/gountz-ia/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/GountzJs/gountz-ia/ci.yml?branch=main&label=CI&style=flat-square)](https://github.com/GountzJs/gountz-ia/actions)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)

*Aísla, orquesta y potencia tus agentes de IA en la terminal con Microkernel Orchy y Git Worktrees.*

</div>

---

## 💡 ¿Qué es `gz-ia`?

**`gz-ia`** (Gountz IA) es un arnés universal de línea de comandos para desarrolladores que ejecutan agentes de inteligencia artificial en su flujo diario de trabajo.

En lugar de dejar que los agentes modifiquen directamente tu espacio de trabajo o colisionen con los archivos que tienes abiertos en el editor, `gz-ia` aprovisiona **árboles de trabajo Git aislados** (`.harness/worktrees/<id>`), ejecuta cualquier agente de terminal (`agy`, `claude`, `opencode`, `pi-agent`), registra su ciclo de vida y te permite inspeccionar y fusionar cambios de manera limpia bajo **control humano**.

### Características Principales

- **Agnóstico Multi-Driver:** Ejecuta indistintamente **Google Antigravity** (`agy`), **Claude Code** (`claude`), **OpenCode** (`opencode`) o **Pi Agent** (`pi-agent`).
- **Aislamiento en Git Worktrees:** Cada sesión trabaja en su propia rama temporal (`harness/<id>`) y directorio desacoplado, sin ensuciar tu rama base ni bloquear tu entorno.
- **Flujo Humano `read` & `get`:** Inspecciona el progreso del agente con `session read` y fusiona cambios controlados a tu rama activa con `session get`.
- **TUI Interactiva + CLI-First:** Navega con una interfaz visual basada en Bubble Tea y formularios Huh (`gz-ia start`), o automatiza todo mediante subcomandos de terminal estándar con salida JSON.
- **Perfiles Componibles (`-P`):** Modela roles de desarrollo (`frontend`, `backend`, `devops`, `security`) inyectando system prompts e instrucciones automáticas.
- **Microkernel Orchy & Servidor MCP:** Servidor Model Context Protocol nativo por Stdio (JSON-RPC 2.0) con protección por Circuit Breaker (Honest Kernel).
- **Vault Seguro de Secretos:** Almacén centralizado (`.harness/vault.json`, permisos `0600`, ignorado por Git) para API keys y variables de entorno, con detección automática de variables faltantes al iniciar sesiones.
- **Observabilidad y Métricas:** Registro estructurado de eventos (`READ` $\to$ `PENDING` $\to$ `FINISH`) y auditoría de consumo de tokens y llamadas a herramientas.
- **Actualizador Integrado:** Comprobación e instalación atómica de nuevas versiones directamente desde GitHub Releases con `gz-ia update`.

---

## 💻 Plataformas Soportadas

Probado y soportado nativamente en arquitecturas **x86_64 / amd64**:

| Plataforma | Arquitectura | Formato de Distribución |
| :--- | :--- | :--- |
| **Linux** | `amd64` | Binario ELF en `.tar.gz` o script one-liner |
| **Windows** | `amd64` | Binario ejecutable `.exe` en `.zip` |

---

## 🚀 Instalación Rápida

### Linux (One-Liner con `curl`)

Instala la última versión en un solo comando:

```bash
curl -fsSL https://raw.githubusercontent.com/GountzJs/gountz-ia/main/install.sh | bash
```

> [!TIP]
> El instalador ubica el binario en `~/.local/bin/gz-ia` (o `/usr/local/bin` si se ejecuta con permisos de superusuario). Asegúrate de tener `~/.local/bin` en tu variable `$PATH`.

### Windows (amd64)

1. Dirígete a la sección de [GitHub Releases](https://github.com/GountzJs/gountz-ia/releases).
2. Descarga el paquete `gz-ia_<version>_windows_amd64.zip`.
3. Descomprime `gz-ia.exe` en la carpeta de tu preferencia (ej. `C:\Tools\gz-ia`).
4. Agrega dicha ruta a tu variable de entorno `PATH`.

### Compilación desde Fuentes (Go 1.23+)

Si tienes instalado el toolchain de Go:

```bash
git clone https://github.com/GountzJs/gountz-ia.git
cd gountz-ia
make build
sudo cp bin/gz-ia /usr/local/bin/
```

Verifica la instalación:

```bash
gz-ia version
```

---

## ⚡ Guía de Uso Rápido

### 1. Interfaz Interactiva (TUI)

Lanza la TUI interactiva para seleccionar agentes, crear perfiles o gestionar sesiones visualmente:

```bash
gz-ia start
# o simplemente:
gz-ia
```

### 2. Iniciar una Sesión Aislada desde la CLI

Lanza una sesión de trabajo con el agente de tu preferencia:

```bash
# Con Google Antigravity CLI
gz-ia chat -p agy -i "Implementar endpoint de autenticación JWT"

# Con Claude Code en modo supervisado
gz-ia chat -p claude -m supervised -i "Refactorizar capa de persistencia"

# Con OpenCode en modo autónomo
gz-ia chat -p opencode -m autonomous -i "Solucionar fallas en tests unitarios"
```

### 3. Usar Perfiles de Especialidad (`-P`)

Define o reutiliza perfiles que inyectan contexto e instrucciones preconfiguradas:

```bash
gz-ia chat -p agy -P frontend -i "Optimizar renderizado de componentes"
gz-ia chat -p agy -P security -i "Auditar dependencias en busca de vulnerabilidades"
```

### 4. Inspección y Fusión Segura de Cambios

Mientras el agente trabaja en su worktree desacoplado:

```bash
# Ver lista de sesiones y rutas de trabajo
gz-ia session list

# Inspeccionar el diff de código generado (sin tocar tu rama activa)
gz-ia session read <session_id>
gz-ia session read <session_id> --stat

# Integrar los cambios producidos al workspace activo
gz-ia session get <session_id> --no-commit

# O fusionar directamente con un commit condensado (squash)
gz-ia session get <session_id> --squash
```

### 5. Vault de Secretos y Variables de Entorno

Almacena de forma segura credenciales de API (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, etc.) sin exponerlas en Git:

```bash
# Listar variables configuradas y recomendadas (ofuscadas por seguridad)
gz-ia vault list

# Guardar una clave (solicita el valor con entrada oculta para no dejar historial)
gz-ia vault set ANTHROPIC_API_KEY

# Inspeccionar estado o revelar temporalmente
gz-ia vault get ANTHROPIC_API_KEY --reveal
```

### 6. Observabilidad, Streaming y Métricas

```bash
# Monitorear eventos en tiempo real mientras el agente trabaja
gz-ia session logs <session_id> -f

# Auditoría forense de duración, tokens consumidos y herramientas ejecutadas
gz-ia session metrics <session_id>
```

### 7. Actualización Automática

Mantén tu instalación de `gz-ia` al día consultando GitHub Releases:

```bash
# Comprobar si existe una nueva versión
gz-ia update --check

# Descargar e instalar la actualización atómicamente
gz-ia update
```

---

## 🛠️ Agentes de Terminal Soportados

| Identificador | Agente | Instalación oficial | Modos Soportados |
| :---: | :--- | :--- | :--- |
| `agy` | **Google Antigravity CLI** | Guía oficial de Antigravity | `readonly` (`--mode plan`), `supervised`, `autonomous` |
| `claude` | **Anthropic Claude Code** | `npm install -g @anthropic-ai/claude-code` | `supervised`, `autonomous` (`--dangerously-skip-permissions`) |
| `opencode` | **OpenCode AI** | `curl -fsSL https://opencode.ai/install \| bash` | `supervised`, `autonomous` |
| `pi-agent` | **Pi Agent CLI** | Documentación oficial de Pi Agent | `supervised`, `autonomous` |

---

## 🧩 Arquitectura del Proyecto

```
gountz-ia/
├── cmd/gz-ia/                  # Punto de entrada de la CLI
├── internal/
│   ├── clients/
│   │   ├── cli/                # Cliente Cobra: comandos chat, session, update, start
│   │   └── tui/                # Cliente interactivo Bubble Tea / Lipgloss / Huh
│   ├── features/
│   │   ├── session/            # Orquestador de sesiones y drivers multi-agente
│   │   ├── workspace/          # Gestor de Git Worktrees, guardrails, diffs y manifiestos
│   │   ├── profile/            # Definición y composición de perfiles agénticos
│   │   ├── tooling/            # Tooling modular, toolkits componibles y proyección en worktree
│   │   ├── vault/              # Almacén seguro de secretos y variables de entorno del proyecto
│   │   ├── logger/             # Event bus y logger append-only (.events.jsonl)
│   │   ├── metrics/            # Agregador de telemetría y analizador de transcripts
│   │   └── updater/            # Actualizador atómico contra GitHub Releases
│   └── version/                # Metadatos de compilación
├── packages/orchy/             # Microkernel extensible con servidor MCP Stdio
│   ├── core/                   # KernelContext, IoC Container y ciclo de vida
│   ├── events/                 # EventBus desacoplado Pub/Sub y RPC
│   ├── tools/                  # Tool Registry y Circuit Breaker (Honest Kernel)
│   ├── mcp/                    # Servidor JSON-RPC 2.0 Model Context Protocol
│   └── batteries/worktree/     # MCP Tools: worktree_read
├── docs/                       # Documentación estática construida con VitePress
└── .github/workflows/          # CI (Go + VitePress + Cloudflare) y Release automatizado
```

---

## 🧪 Pruebas y Desarrollo

Para ejecutar la suite de pruebas unitarias con detección de carreras (*race detector*):

```bash
# Ejecutar todas las pruebas en Go
go test -v -race -count=1 ./...

# Compilar binario local
make build

# Iniciar servidor local de documentación VitePress
make docs-dev

# Compilar documentación estática para producción
make docs-build
```

---

## 📄 Licencia

Este proyecto está distribuido bajo la licencia [MIT](LICENSE).

---

<div align="center">
Desarrollado con pasión por <b>GountzJs</b>
</div>
