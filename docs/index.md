---
layout: home

hero:
  name: "Gountz IA"
  text: "Declara tu stack una vez. Úsalo con cualquier agente de IA."
  tagline: "gz-ia — Comparte toolkits modulares (React, Node, Tailwind, reglas y MCP) entre Antigravity, Claude Code y OpenCode, con control de cambios limpio en Git."
  image:
    src: /logo.png
    alt: Gountz IA Logo
  actions:
    - theme: brand
      text: Inicio Rápido
      link: /guide/quickstart
    - theme: alt
      text: ¿Para quién es? & FAQ
      link: /guide/faq
    - theme: alt
      text: Toolkits y Presets
      link: /guide/profiles
    - theme: alt
      text: Estado y Limitaciones
      link: /guide/limitations

features:
  - icon: ⚙️
    title: Toolkits Modulares Reutilizables
    details: Declara directivas maestras (`AGENTS.md`), reglas técnicas (`rules/`), procedimientos (`skills/`) y herramientas MCP una sola vez. Se proyectan al vuelo en cualquier CLI.
    link: /guide/profiles
  - icon: ⇄
    title: Cambia de Agente sin Reconfigurar
    details: Alterna libremente entre Google Antigravity, Claude Code u OpenCode sin tener que volver a enseñarle tus convenciones, linters ni librerías a cada agente.
    link: /guide/providers
  - icon: ⎇
    title: Archivos Activos Aislados
    details: Cada sesión trabaja en su propio Git Worktree (`.harness/worktrees/<id>`). El agente puede probar y compilar en un árbol paralelo sin alterar tus archivos abiertos ni tu `git status`.
    link: /architecture/worktrees
  - icon: ✓
    title: Revisión Humana en un Comando
    details: Inspecciona con `gz-ia session diff` e integra las modificaciones validadas a tu rama activa con `gz-ia session get`. Tú decides qué entra y qué no.
    link: /architecture/worktrees
  - icon: ⚿
    title: Variables de Entorno y Secretos (Vault)
    details: Almacena credenciales locales en `.harness/vault.json` (permisos 0600) e inyección en el entorno del agente, excluido del seguimiento de Git.
    link: /guide/vault
  - icon: ⌂
    title: 100% Local y Cero Telemetría
    details: Sin servidores externos, sin cuentas de terceros y sin telemetría. Todo vive exclusivamente en tu máquina y en la carpeta `.harness/` de tu proyecto.
    link: /guide/faq
---

## El Problema que Resuelve gz-ia

Cada agente de terminal (Claude Code, Google Antigravity, OpenCode, Pi Agent) utiliza su propio esquema para configurar directivas, reglas y herramientas. Si alternas entre agentes o trabajas en equipo:
1. **Reescribes el contexto:** Explicas repetidamente a cada agente las convenciones de carpetas, linters y estándares.
2. **Duplicación de configuraciones:** Copias carpetas `.agents/` o `rules/` a mano entre múltiples proyectos, desactualizándose de inmediato.
3. **Colisiones en el editor:** Los agentes modifican archivos en caliente en el directorio raíz, alterando tu `git status` y disparando recargas en servidores de desarrollo.

**La propuesta de `gz-ia` es simple:**
> **"Declara una sola vez qué significa trabajar en tu stack y úsalo exactamente igual con cualquier agente de IA."**

---

## ¿Para quién es gz-ia? (Y para quién no)

### Es para ti si:
- **Alternas entre múltiples agentes:** Usas Antigravity para razonamiento profundo, Claude Code para refactorizaciones u OpenCode para scripts, y quieres que todos compartan el mismo contexto.
- **Trabajas en equipo:** Quieres que los miembros del equipo y sus agentes sigan las mismas reglas de arquitectura y herramientas MCP compartidas en el repositorio (`.gz-ia/toolkits/`).
- **Valoras el control de cambios:** Quieres que el agente trabaje en un árbol paralelo sin alterar tus archivos activos hasta que revises el diff y decidas integrarlo.

### No lo necesitas si:
- Usas **un solo agente de terminal**, trabajas en un proyecto pequeño y el flujo de edición directa en tu espacio de trabajo te resulta suficiente.

---

## Instalación en un Solo Comando

::: code-group

```bash [Linux (Recomendado)]
curl -fsSL https://raw.githubusercontent.com/GountzJs/gountz-ia/main/install.sh | bash
```

```bash [Prueba Rápida]
# Lanzar chat interactivo con el primer agente disponible
gz-ia chat

# O abrir la interfaz gráfica de terminal (TUI)
gz-ia start
```

:::

---

## Estado del Proyecto

`gz-ia` se encuentra en **versión v0.1.0**. El aislamiento en Git y la proyección de toolkits son operativos en **Linux (amd64)** y en validación para **macOS**. En **Windows** existen restricciones de bloqueo de archivos al actualizar binarios en caliente.

Consulta [Estado y Limitaciones Conocidas](/guide/limitations) y [Preguntas Frecuentes](/guide/faq).
