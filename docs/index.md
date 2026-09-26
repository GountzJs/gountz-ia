---
layout: home

hero:
  name: "Gountz IA"
  text: "Universal Terminal AI Harness"
  tagline: "gz-ia — Aísla, orquesta y potencia tus agentes de IA en la terminal con Microkernel Orchy y Git Worktrees"
  image:
    src: /logo.png
    alt: Gountz IA Logo
  actions:
    - theme: brand
      text: Inicio Rápido
      link: /guide/quickstart
    - theme: alt
      text: Arquitectura
      link: /architecture/overview
    - theme: alt
      text: Microkernel Orchy
      link: /orchy/microkernel
    - theme: alt
      text: Referencia CLI
      link: /reference/cli

features:
  - icon: 🤖
    title: Agnóstico Multi-Driver
    details: Orquesta agentes de terminal (`agy`, `claude code`, `opencode`, `pi-agent`) traduciendo banderas y modos de interacción a las capacidades reales de cada CLI subyacente.
    link: /guide/providers
  - icon: 🌳
    title: Desacoplamiento con Git Worktrees
    details: Cada sesión opera en un working tree independiente (`.harness/worktrees/<id>`). Evita interferencias con tus archivos abiertos en el editor mientras trabajas en paralelo.
    link: /architecture/worktrees
  - icon: ⚡
    title: Microkernel Orchy & MCP
    details: Microkernel interno ligero para el servidor MCP JSON-RPC 2.0 por Stdio, Circuit Breaker (`HEALTHY`, `DEGRADED`, `DEAD`) y extensiones modulares.
    link: /orchy/microkernel
  - icon: 💻
    title: TUI Bubble Tea + CLI-First
    details: Interfaz interactiva de terminal construida con Bubble Tea y formularios Huh, con el 100% de capacidades automatizables mediante comandos directos de CLI.
    link: /guide/tui
  - icon: 📊
    title: Observabilidad Local
    details: Registro estructurado de etapas de ciclo de vida (`READ` → `PENDING` → `FINISH`) en `.events.jsonl`, con telemetría de proceso y métricas de tokens.
    link: /architecture/observability
  - icon: 🚀
    title: Distribución & Actualización
    details: Instalación directa en un comando mediante `curl` desde GitHub Releases y actualización transparente in-place mediante `gz-ia update`.
    link: /guide/quickstart
---

## ⚡ Diferencial Clave de gz-ia

En lugar de lanzar agentes directamente en tu directorio de trabajo o memorizar banderas incompatibles, `gz-ia` proporciona tres capacidades fundamentales:

1. **Flujo uniforme de revisión e integración:** Sin importar si usas `agy` o `claude`, el ciclo de inspección e integración en Git (`read` → `diff` → `get`) es idéntico y controlado por el desarrollador.
2. **Espacio de trabajo desacoplado:** Ejecución de múltiples sesiones agénticas en paralelo sin colisionar en los archivos abiertos del editor ni ensuciar tu `git status`.
3. **Observabilidad y registro unificado local:** Trazabilidad de etapas y auditoría en `.harness/` sin dependencias de servicios externos.

::: tip Instalación universal en un solo comando
```bash
curl -fsSL https://raw.githubusercontent.com/GountzJs/gountz-ia/main/install.sh | bash
```
:::

::: tip Prueba rápida en un solo comando
```bash
# Lanzar un chat interactivo con el agente disponible
gz-ia chat --provider agy --perm supervised

# O abrir la interfaz visual interactiva de terminal (TUI)
gz-ia start
```
:::

