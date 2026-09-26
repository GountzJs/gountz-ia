# Historial de Versiones (Changelog)

Todos los cambios notables en este proyecto serán documentados en este archivo.

El formato está basado en [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/) y este proyecto se adhiere a [Semantic Versioning](https://semver.org/lang/es/).

---

## [0.0.1] - 2026-09-24

### Lanzamiento Inicial — Fundación del Harness

Esta versión marca la primera entrega oficial de `gz-ia`, estableciendo las bases de la infraestructura universal de terminal para agentes de inteligencia artificial.

#### ✨ Características Principales

- **Arquitectura Agnóstica Multi-Driver:**
  - Soporte de primer orden para 4 agentes de terminal: Google Antigravity (`agy`), Anthropic Claude Code (`claude`), OpenCode (`opencode`) y Pi Agent (`pi-agent`).
  - Normalización de permisos en tres niveles: `readonly`, `supervised` y `autonomous`.
  - Detección automática en vivo de binarios con comandos sugeridos de instalación.
- **Aislamiento en Git Worktrees Efímeros:**
  - Creación automática de árboles de trabajo en `.harness/worktrees/<id>` vinculados a ramas dedicadas `harness/<id>`.
  - Inspección limpia de diferencias de código con `session diff` y `session read`.
  - Integración atómica hacia la rama principal con `session get` (`--squash`, `--no-commit`).
  - Fallback transparente sin interrupción en directorios donde Git no esté inicializado.
- **Microkernel Orchy & Servidor MCP:**
  - Contenedor IoC de servicios (`ServiceContainer`) y bus tipado de eventos (`EventBus`).
  - Patrón Circuit Breaker de 3 estados (`HEALTHY`, `DEGRADED`, `DEAD`) sobre proxies de herramientas (`ToolProxy`).
  - Servidor MCP nativo sobre Stdio con especificación JSON-RPC 2.0 y generación automática de manifiestos.
  - Baterías de worktrees incluidas (`worktree_read` por defecto para inspección del agente, `worktree_get` para integración controlada).
- **TUI Interactiva con Fastfetch & CLI-First:**
  - Interfaz visual accesible construida con Bubble Tea, Lipgloss y formularios Huh.
  - Paridad funcional del 100% entre la TUI y la línea de comandos.
- **Observabilidad y Telemetría:**
  - Logger estructurado de etapas (`READ` $\rightarrow$ `PENDING` $\rightarrow$ `FINISH`) persistido en formato `.events.jsonl`.
  - Agregador de métricas con cálculo de duración, recuento de pasos y desglose de tokens y llamadas a herramientas.
- **Actualizador Atómico de GitHub Releases:**
  - Verificación continua e instalación atómica de releases binarias desde GitHub Releases.
