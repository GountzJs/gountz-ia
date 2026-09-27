---
layout: home

hero:
  name: "Gountz IA"
  text: "Declará tu stack una vez. Usalo con cualquier agente de IA."
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
  - icon: 🧩
    title: Toolkits Modulares Reutilizables
    details: Declara tus directivas maestras (`AGENTS.md`), reglas técnicas (`rules/`), procedimientos (`skills/`) y herramientas MCP una sola vez. Se proyectan al vuelo en cualquier CLI.
    link: /guide/profiles
  - icon: 🔄
    title: Cambia de Agente sin Reconfigurar
    details: Alterna libremente entre Google Antigravity, Claude Code u OpenCode sin tener que volver a enseñarle tus convenciones, linters ni librerías a cada agente.
    link: /guide/providers
  - icon: 🛡️
    title: Tus Archivos Activos Intactos
    details: Cada sesión trabaja en su propio Git Worktree (`.harness/worktrees/<id>`). El agente puede probar, romper y compilar sin congelar tu editor ni ensuciar tu `git status`.
    link: /architecture/worktrees
  - icon: 🔍
    title: Revisión Humana en Un Comando
    details: Inspecciona con `gz-ia session diff` y trae las modificaciones validadas a tu rama activa con `gz-ia session get`. Vos decidís qué entra y qué no.
    link: /architecture/worktrees
  - icon: 🔐
    title: Secretos y Envs Centralizados
    details: Almacena variables de entorno requeridas en `.harness/vault.json` con permisos `0600` e inyección transparente en cada sesión sin exponerlas en git.
    link: /guide/vault
  - icon: 🌐
    title: 100% Local y Cero Telemetría
    details: Sin servidores externos, sin cuentas de terceros y sin telemetría. Todo vive exclusivamente en tu máquina y en la carpeta `.harness/` de tu repositorio.
    link: /guide/faq
---

## ⚡ El Problema Real que Resuelve gz-ia

Hoy cada agente de terminal (Claude Code, Google Antigravity, OpenCode, Pi Agent) inventa su propia forma de configurar directivas, reglas y herramientas. Si cambias de agente o trabajas en equipo:
1. **Reescribes el contexto una y otra vez:** Vuelves a explicarle a cada agente tus convenciones de carpetas, linters y estándares.
2. **Duplicación caótica:** Copias carpetas `.agents/` o `rules/` a mano entre múltiples proyectos, desactualizándose de inmediato.
3. **Colisiones en tu editor:** Los agentes modifican tus archivos en caliente, rompiendo tu `git status` y disparando recargas molestas de desarrollo.

**La propuesta de `gz-ia` es simple:**
> **"Declara una sola vez qué significa trabajar en tu stack (ej. React Native + Tailwind) y úsalo exactamente igual con cualquier agente de IA."**

---

## 🎯 ¿Para quién es gz-ia? (Y para quién no)

### ✅ Es para vos si:
- **Alternas entre múltiples agentes:** Usas Antigravity para razonamiento profundo, Claude Code para refactors u OpenCode para scripts, y quieres que todos compartan el mismo contexto.
- **Trabajas en equipo:** Quieres que todos los miembros del equipo y sus agentes sigan las mismas reglas de arquitectura y herramientas MCP compartidas en el repositorio.
- **Valoras el control de cambios:** Quieres que el agente trabaje en un árbol paralelo sin alterar tus archivos abiertos hasta que revises el diff y decidas integrarlo.

### ❌ No lo necesitas si:
- Usas **un solo agente de terminal**, trabajas solo en un proyecto pequeño y el flujo directo del CLI ya te resulta suficiente.

---

## 🚀 Instalación en un Solo Comando

::: code-group

```bash [Linux (Recomendado)]
curl -fsSL https://raw.githubusercontent.com/GountzJs/gountz-ia/main/install.sh | bash
```

```bash [Prueba Rápida]
# Lanzar chat interactivo con el agente disponible
gz-ia chat

# O abrir la interfaz gráfica de terminal (TUI)
gz-ia start
```

:::

---

## ⚠️ Transparencia: Versión Alfa Activa

`gz-ia` se encuentra en **fase alfa activa (v0.0.3)**. El aislamiento en Git y la proyección de toolkits son completamente operativos en **Linux (amd64)**. El soporte nativo para **macOS** está en desarrollo y la actualización en caliente en **Windows** cuenta con restricciones del sistema de archivos.

Te invitamos a leer nuestra página de [Estado y Limitaciones Conocidas](/guide/limitations) y las [Preguntas Frecuentes](/guide/faq) antes de incorporarlo en flujos de producción.
