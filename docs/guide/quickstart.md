# Inicio Rápido (Quickstart)

Esta guía te conducirá paso a paso para instalar, verificar y ejecutar tu primera sesión de desarrollo con `gz-ia`.

---

## 1. Prerrequisitos y Plataformas

- **Plataformas Soportadas:**
  - **Linux:** Distribuciones modernas de 64 bits (Ubuntu, Debian, Fedora, Arch, etc.).
  - **Windows:** Soporte para Windows (amd64 / x86_64).
- **Git:** Versión `2.20+` con soporte de `git worktree`.
- **Al menos un agente de terminal instalado:**
  - [Google Antigravity (`agy`)](/guide/providers#google-antigravity-agy)
  - [Anthropic Claude Code (`claude`)](/guide/providers#anthropic-claude-code-claude)
  - [OpenCode (`opencode`)](/guide/providers#opencode-opencode)
  - [Pi Agent (`pi-agent`)](/guide/providers#pi-agent-pi-agent)

---

## 2. Instalación

::: code-group

```bash [GitHub (Recomendado)]
# Instalación automática para Linux (amd64)
curl -fsSL https://raw.githubusercontent.com/GountzJs/gountz-ia/main/install.sh | bash
```

```bash [Compilación desde código (Make)]
# Requiere Go 1.23+
git clone https://github.com/GountzJs/gountz-ia.git
cd gountz-ia
make build
sudo cp bin/gz-ia /usr/local/bin/
```

:::

::: tip Actualización de instalaciones existentes
Una vez que dispones de `gz-ia` instalado, puedes verificar y aplicar nuevas versiones directamente con el comando de auto-actualización:
```bash
# Comprobar si hay versiones más recientes en GitHub Releases
gz-ia update --check

# Descargar e instalar la actualización in-place
gz-ia update
```
:::

---

## 3. Comprobación de Instalación

Comprueba que el binario responda adecuadamente con su versión:

```bash
$ gz-ia version
✦ gz-ia v0.0.2 (commit: ceed405, date: 2026-09-26T20:40:19Z)
```

---

## 4. Lanzar una Sesión de Chat

Para iniciar una sesión interactiva directa conectada al primer motor de IA detectado:

```bash
# Detecta el primer agente disponible en PATH
gz-ia chat
```

### Especificar Motor, Nivel de Permiso y Objetivo

```bash
# Ejecutar Google Antigravity en modo supervisado con un prompt inicial
gz-ia chat -p agy -m supervised -i "Añadir validación de email con pruebas unitarias"
```

**Salida en terminal:**

```text
🚀 Iniciando chat con Google Antigravity (Permiso: supervised | Workspace: Worktree desacoplado (.harness/worktrees/7e2a9b1c))...
```

**Mecanismo interno:**
1. `gz-ia` genera un identificador aleatorio de sesión (ej. `7e2a9b1c`).
2. Crea un **Git Worktree efímero** en `.harness/worktrees/7e2a9b1c` sobre una rama dedicada `harness/7e2a9b1c`.
3. El agente opera dentro de ese worktree; tus archivos abiertos en el editor permanecen inalterados.

---

## 5. Inspeccionar y Traer Cambios

Cuando el agente complete su trabajo, puedes auditar las modificaciones antes de integrarlas:

### 1. Listar las sesiones

```bash
$ gz-ia session list
ID          ESTADO        PID       PERMISO       MODO        INICIO                DURACIÓN  
──────────────────────────────────────────────────────────────────────────────────────────
7e2a9b1c    RUNNING       41892     supervised    Worktree    2026-09-25 00:30:15   4m 12s    
3f9a12c8    COMPLETED     -         autonomous    Worktree    2026-09-25 00:15:02   2m 45s    
```

### 2. Inspeccionar el resumen estadístico de cambios (`--stat`)

```bash
$ gz-ia session read 7e2a9b1c --stat
 internal/validator/email.go      | 42 ++++++++++++++++++++++++++++++++++++++++++
 internal/validator/email_test.go | 58 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 2 files changed, 100 insertions(+)
```

### 3. Visualizar las diferencias completas de código

```bash
$ gz-ia session diff 7e2a9b1c
```

### 4. Integrar las modificaciones al workspace activo

La integración de código es una acción exclusiva del desarrollador:

```bash
$ gz-ia session get 7e2a9b1c --no-commit
✓ Cambios del worktree traídos e integrados con éxito para la sesión '7e2a9b1c'.
Archivos integrados:
  • internal/validator/email.go
  • internal/validator/email_test.go
Nota: Los cambios quedaron preparados en el stage sin comitear (--no-commit).
```

---

## 6. Consideraciones Prácticas de Entorno

### Exclusión de `.harness/` en herramientas de análisis
Para evitar que linters, Jest, compiladores TypeScript o watchers de IDE indexen los archivos duplicados en los worktrees:
- **`tsconfig.json`:** Agrega `".harness"` al array `"exclude"`:
  ```json
  {
    "exclude": ["node_modules", ".harness"]
  }
  ```
- **Exclusión Git:** `gz-ia` registra automáticamente `.harness/` en `.git/info/exclude` del repositorio local para que permanezca ignorado sin ensuciar tu archivo `.gitignore` compartido.
- **ESLint / IDE Watchers / Vite:** Incluye `.harness/**` en los patrones ignorados (por ejemplo, `server.watch.ignored: ['**/.harness/**']` en Vite) para prevenir recargas innecesarias.

### Gestión de Dependencias (Proyectos Node / Frontend)
Dado que cada worktree es un árbol de archivos separado, no comparte automáticamente la carpeta `node_modules`:
- En proyectos con dependencias pesadas, se recomienda usar gestores con cache y enlaces globales como **pnpm** o **bun** para evitar instalaciones lentas.
- Opcionalmente, puedes crear un symlink al `node_modules` de la raíz subiendo tres niveles (`ln -s $(gz-ia session path <id>)/../../../node_modules $(gz-ia session path <id>)/node_modules`). Asegúrate de que tu `.gitignore` tenga `node_modules` sin barra final para no commitear el symlink.

---

## 7. Interfaz Visual Interactiva (TUI)

Si prefieres navegar tus sesiones con interfaz de terminal:

```bash
gz-ia start
# o simplemente
gz-ia
```
