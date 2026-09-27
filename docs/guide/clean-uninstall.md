# Limpieza y Desinstalación

Nadie piensa en cómo desinstalar una herramienta hasta que necesita limpiar su repositorio. `gz-ia` fue diseñado con el principio de **cero contaminación**: todo lo que genera vive en rutas estrictamente delimitadas que puedes auditar y eliminar en segundos.

Esta guía explica qué archivos crea `gz-ia`, cómo limpiar sesiones residuales y cómo retirar la herramienta por completo de tu proyecto o sistema.

---

## Qué genera `gz-ia` en tu proyecto

Cuando utilizas `gz-ia` en un repositorio, únicamente se crean las siguientes entidades locales:

1. **La carpeta `.harness/`:**
   - `.harness/worktrees/<id>`: Directorios de trabajo efímeros para cada sesión.
   - `.harness/sessions/<id>.json`: Metadata estructurada de cada sesión (PID, duración, proveedor).
   - `.harness/sessions/<id>.events.jsonl`: Registro de eventos y observabilidad de sesión.
   - `.harness/sessions/<id>.manifest.json`: Manifiesto de archivos proyectados para fusiones no destructivas.
   - `.harness/toolkits/`: Toolkits modulares declarados a nivel de proyecto.
   - `.harness/vault.json`: Variables de entorno y secretos locales (con permisos `0600`).
2. **Ramas locales de Git:**
   - Ramas temporales con el patrón `harness/<id>` asociadas a cada sesión de trabajo.
3. **Referencias de Worktrees en Git:**
   - Metadatos internos de Git ubicados en `.git/worktrees/harness-<id>`.

---

## 1. Limpieza de Sesiones Huérfanas (`prune`)

Si una sesión se interrumpió de forma anómala (ej. la terminal se cerró abruptamente o el sistema se reinició), puedes reconciliar y eliminar automáticamente todas las ramas y worktrees huérfanos con un solo comando:

```bash
gz-ia session prune
```

Salida esperada:
```text
✧ Reconciliación de Basura Huérfana:
  ✓ Rama huérfana eliminada: harness/a1b2c3d4
  ✓ Worktree huérfano podado: .harness/worktrees/a1b2c3d4
```

---

## 2. Eliminar `gz-ia` por Completo de tu Repositorio

Si deseas remover todo rastro de `gz-ia` de tu proyecto y dejar el repositorio exactamente como estaba antes de usarlo:

```bash
# 1. Poda y desvincula los worktrees en Git
git worktree prune

# 2. Elimina todas las ramas locales temporales creadas por sesiones
git branch -D $(git branch --list 'harness/*') 2>/dev/null || true

# 3. Elimina la carpeta del arnés
rm -rf .harness/
```

Una vez ejecutados estos comandos, tu repositorio no conservará ningún rastro ni metadato de `gz-ia`.

---

## 3. Desinstalación del Binario del Sistema

Si deseas remover el binario de `gz-ia` de tu máquina:

```bash
# Si se instaló en ~/.local/bin (predeterminado de install.sh):
rm -f ~/.local/bin/gz-ia

# Si se instaló globalmente en /usr/local/bin:
sudo rm -f /usr/local/bin/gz-ia

# Para eliminar la configuración global de usuario (toolkits globales y presets):
rm -rf ~/.config/gz-ia/
```
