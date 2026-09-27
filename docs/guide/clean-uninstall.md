# Limpieza y Desinstalación

Nadie piensa en cómo desinstalar una herramienta hasta que necesita limpiar su repositorio. `gz-ia` fue diseñado para delimitar todo lo que genera en rutas predecibles que puedes auditar y eliminar en segundos.

Esta guía explica qué archivos crea `gz-ia`, cómo limpiar sesiones residuales y cómo retirar la herramienta por completo de tu proyecto o sistema.

---

## Qué genera `gz-ia` en tu proyecto

Cuando utilizas `gz-ia` en un repositorio, se crean las siguientes entidades locales:

1. **La carpeta `.harness/`:**
   - `.harness/worktrees/<id>`: Directorios de trabajo efímeros para cada sesión.
   - `.harness/sessions/<id>.json`: Metadata estructurada de cada sesión (PID, duración, proveedor).
   - `.harness/sessions/<id>.events.jsonl`: Registro de eventos y observabilidad de sesión.
   - `.harness/sessions/<id>.manifest.json`: Manifiesto de archivos proyectados para fusiones no destructivas.
   - `.harness/toolkits/`: Toolkits declarados localmente en el arnés (a diferencia de `.gz-ia/toolkits/` que viajan con el repositorio en Git).
   - `.harness/vault.json`: Variables de entorno y secretos locales (con permisos `0600`).
2. **Ramas locales de Git:**
   - Ramas temporales con el patrón `harness/<id>` asociadas a cada sesión de trabajo.
3. **Referencias de Worktrees y Configuración en Git:**
   - Metadatos internos de Git en `.git/worktrees/harness-<id>`.
   - Exclusión local en `.git/info/exclude` (`.harness`).
   - Flag de configuración `extensions.worktreeConfig` si Git lo requirió.

---

## 1. Limpieza de Sesiones Huérfanas (`prune`)

Si una sesión se interrumpió de forma anómala (por ejemplo, la terminal se cerró abruptamente o el sistema se reinició), puedes reconciliar y eliminar automáticamente todas las ramas y worktrees huérfanos con un solo comando:

```bash
gz-ia session prune
```

Salida esperada:
```text
Reconciliación de Basura Huérfana:
  ✓ Rama huérfana eliminada: harness/a1b2c3d4
  ✓ Worktree huérfano podado: .harness/worktrees/a1b2c3d4
```

---

## 2. Eliminar `gz-ia` por Completo de tu Repositorio

Si deseas remover todo rastro de `gz-ia` de tu proyecto y devolver el repositorio a su estado previo:

```bash
# 1. Poda y desvincula las referencias de worktrees en Git
git worktree prune

# 2. Elimina todas las ramas locales creadas por sesiones de gz-ia
# Usamos for-each-ref para evitar fallos con símbolos '+' o '*' en listas de ramas
git for-each-ref --format='%(refname:short)' refs/heads/harness/ | xargs -r git branch -D
```

> [!WARNING] Advertencia antes de borrar `.harness/`
> El siguiente paso borrará de forma irreversible tu Vault local (`.harness/vault.json`) y cualquier toolkit que no hayas guardado en `.gz-ia/toolkits/` o en tu configuración global. Si necesitas preservar tus claves o configuraciones locales, haz una copia de respaldo antes de ejecutar:

```bash
# 3. Elimina la carpeta del arnés
rm -rf .harness/

# 4. Remueve la exclusión local en Git y la configuración de worktree
sed -i '/\.harness/d' .git/info/exclude 2>/dev/null || true
git config --unset extensions.worktreeConfig 2>/dev/null || true
```

Tras ejecutar estos pasos, tu repositorio no conservará ninguna configuración residual ni rastro de `gz-ia`.

---

## 3. Desinstalación del Binario del Sistema

Si deseas remover el binario de `gz-ia` de tu máquina:

```bash
# Si se instaló en ~/.local/bin (predeterminado de install.sh):
rm -f ~/.local/bin/gz-ia

# Si se instaló globalmente en /usr/local/bin:
sudo rm -f /usr/local/bin/gz-ia

# Para eliminar la configuración global de usuario (presets y toolkits globales):
rm -rf ~/.config/gz-ia/
```
