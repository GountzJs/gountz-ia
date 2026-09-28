# Preguntas Frecuentes (FAQ)

Preguntas frecuentes sobre arquitectura, seguridad, compatibilidad y flujos de trabajo en `gz-ia`.

---

## 1. ¿Qué me da que no tenga usando Claude Code o Antigravity directo?

> [!TIP] En síntesis
> **Un flujo uniforme de revisión y control en Git independiente del agente utilizado, y toolkits modulares reutilizables (directivas, reglas, skills y MCP) proyectados en cualquier CLI.**

Si trabajas solo, utilizas un único agente de terminal y estás conforme con que modifique directamente tu directorio de trabajo, **probablemente no necesites `gz-ia`**.

`gz-ia` cobra valor real en dos escenarios:
1. **Alternas entre múltiples agentes:** Quieres usar Antigravity para razonamiento profundo, Claude Code para refactorizaciones rápidas u OpenCode para automatizaciones, sin tener que volver a enseñarle tus linters, convenciones de arquitectura y herramientas MCP a cada herramienta por separado.
2. **Trabajas en equipo:** Quieres que todo el equipo comparta los mismos toolkits de stack (ej. React Native + Tailwind) **versionados en el repositorio** bajo `.gz-ia/toolkits/`, garantizando que cualquier agente que levante un desarrollador opere bajo exactamente las mismas reglas compartidas por Git.

---

## 2. ¿Es seguro? ¿El agente queda aislado?

**No es un sandbox de sistema operativo.**

`gz-ia` aísla el **árbol de trabajo de Git (Working Tree)** mediante `git worktree`, lo que garantiza que:
- Tu editor no se congela ni se desincroniza mientras el agente edita código en paralelo.
- Tu rama activa y tu `git status` no se alteran.
- Un cambio destructivo en el código del agente queda contenido en una rama efímera (`harness/<id>`) hasta que decidas integrarlo.

**Sin embargo:**
- El agente se ejecuta con los **privilegios de tu usuario** en el sistema operativo.
- Si le das permisos autónomos (`--perm autonomous` o `--dangerously-skip-permissions`), el agente puede ejecutar comandos de terminal en tu máquina.
- Los guardrails de Git previenen accidentes cotidianos de sobreescritura, pero no contienen a un proceso malicioso o comandos destructivos intencionales (`rm -rf ~`). Para contención total a nivel de kernel, utiliza contenedores Docker o DevContainers.

---

## 3. ¿Tengo que reinstalar `node_modules` en cada sesión?

Al crear un Git Worktree aislado (`.harness/worktrees/<id>`), Git recrea los archivos rastreados en el repositorio. Los directorios incluidos en `.gitignore` (como `node_modules`, `target/` en Rust o `vendor/`) **no existen inicialmente en el nuevo worktree**.

Si tu agente necesita ejecutar tests o linters que dependen de `node_modules`, tienes estas alternativas prácticas:

1. **Enlace simbólico hacia la raíz (Workaround recomendado):**
   Crea un symlink desde el worktree hacia el `node_modules` de tu directorio base (subiendo tres niveles: `worktrees/<id>` -> `worktrees` -> `.harness` -> raíz del proyecto):
   ```bash
   ln -s $(gz-ia session path <id>)/../../../node_modules $(gz-ia session path <id>)/node_modules
   ```
   > [!WARNING] Cuidado con la barra diagonal en `.gitignore`
   > Si el `.gitignore` de tu proyecto define `node_modules/` (con barra final), Git interpreta la regla únicamente para carpetas reales. Para Git, un enlace simbólico es un archivo de tipo symlink, no un directorio. Por lo tanto, `node_modules/` no lo ignorará y `gz-ia session get` intentará commitear el symlink. Para evitarlo, asegúrate de que en `.gitignore` figure simplemente `node_modules` (sin barra al final) o ignora explícitamente el symlink.

2. **Cuidado con Watchers de Desarrollo (Vite, Webpack, Tailwind):**
   Dado que los worktrees residen en `.harness/worktrees/<id>` dentro del proyecto, herramientas de frontend que vigilan todo el directorio pueden detectar las modificaciones del agente y disparar recargas o escaneos duplicados de clases CSS.
   - En **Vite** (`vite.config.ts`), añade la exclusión:
     ```ts
     server: {
       watch: {
         ignored: ['**/.harness/**']
       }
     }
     ```
   - En **Tailwind**, restringe el escaneo a rutas específicas (ej. `./src/**/*.{ts,tsx}`) en lugar de globs genéricos sobre la raíz (`./**/*.{ts,tsx}`).

3. **Gestores con caché centralizada (pnpm / bun):**
   Con `pnpm install` o `bun install` dentro del worktree, la resolución tarda segundos gracias a los enlaces duros compartidos en disco.

4. **Modo directo sin aislamiento:**
   Si estás realizando una consulta rápida o no necesitas aislamiento de rama, puedes correr el agente directamente en tu directorio raíz:
   ```bash
   gz-ia chat -p agy -d .
   ```

---

## 4. ¿Funciona en macOS? ¿Y en Windows?

Compatibilidad por plataforma en la versión actual (v0.4.0):

| Plataforma | Estado | Detalle |
| :--- | :--- | :--- |
| **Linux (x86_64 / amd64)** | **Soportado** | Plataforma primaria de desarrollo. Todas las características operativas. |
| **macOS (Apple Silicon / Intel)** | **En estabilización** | Compila y ejecuta. |
| **Windows (amd64)** | **Parcial** | La ejecución de sesiones y la TUI funcionan. La actualización atómica en caliente (`gz-ia update`) no reemplaza el binario en memoria debido al bloqueo de archivos de Win32. Se recomienda descargar los paquetes `.zip` directamente desde GitHub Releases. |

---

## 5. ¿Necesito una API Key? ¿Usa mi suscripción?

**`gz-ia` no requiere cuentas externas ni servidores de autenticación propios.**

`gz-ia` es un arnés local que interactúa con las CLIs instaladas en el sistema (`agy`, `claude`, `opencode`, `pi-agent`).
- Si usas **Claude Code** con suscripción oficial Pro o Team (vía `claude login`), `gz-ia` utiliza directamente la sesión autenticada existente. **No es necesario configurar `ANTHROPIC_API_KEY`** (si se define en el vault, Claude Code prioriza la API key y factura por consumo de tokens).
- Si usas **Antigravity** con tu cuenta de Google, se ejecuta directamente contra la sesión local autenticada.
- Si usas **OpenCode**, se conecta a los proveedores configurados en el entorno local.

---

## 6. ¿Manda algo afuera? ¿Tiene telemetría?

**No incluye telemetría ni llamadas a servicios externos.**

- No recopila analíticas ni métricas de uso remoto.
- Toda la metadata de ejecución, trazas y métricas se almacena localmente en la carpeta `.harness/` del proyecto.
- El código es abierto bajo licencia MIT.

---

## 7. ¿Dónde quedan mis secretos?

Las variables de entorno y claves gestionadas con `gz-ia vault` se almacenan localmente en:
```text
.harness/vault.json
```

**Condiciones de seguridad del Vault:**
- **Permisos estrictos:** Se guarda con permisos POSIX `0600` (lectura y escritura exclusivas para tu usuario del sistema operativo).
- **Protección Git:** La carpeta `.harness/` se añade automáticamente a `.git/info/exclude` del repositorio local para prevenir commits accidentales.
- **Formato:** Es un archivo JSON en texto plano en tu disco local. No cuenta con cifrado criptográfico simétrico en reposo con contraseña maestra.
- **Alcance en sesiones:** Al iniciar una sesión, las variables del vault se inyectan en el entorno (`env`) del proceso del agente, y el archivo físico `.harness/vault.json` se encuentra accesible desde el worktree mediante la ruta relativa `../../vault.json`.

---

## 8. ¿Cómo lo saco de mi repositorio?

`gz-ia` está diseñado para no dejar basura permanente en tu proyecto. Para remover todo rastro:

1. **Reconcilia y elimina sesiones activas:**
   ```bash
   gz-ia session prune
   ```

2. **Advertencia de respaldo de secretos:**
   > [!WARNING]
   > Al eliminar la carpeta `.harness/` se borrarán tu archivo de secretos local (`.harness/vault.json`) y cualquier toolkit que no hayas guardado en `.gz-ia/toolkits/`. Si necesitas conservarlos, cópialos a otra ubicación antes de continuar.

3. **Elimina la carpeta local de arnés:**
   ```bash
   rm -rf .harness/
   ```

4. **Elimina ramas temporales de sesiones:**
   ```bash
   git for-each-ref --format='%(refname:short)' refs/heads/harness/ | xargs -r git branch -D
   ```

5. **Limpia exclusiones y configuración de Git:**
   Si deseas restaurar la configuración interna de Git exactamente a su estado original, remueve la línea `.harness` de `.git/info/exclude` y desactiva la extensión si quedó configurada:
   ```bash
   sed -i '/\.harness/d' .git/info/exclude 2>/dev/null || true
   git config --unset extensions.worktreeConfig 2>/dev/null || true
   ```

Consulta la [Guía de Limpieza y Desinstalación](/guide/clean-uninstall) para más detalles.

---

## 9. ¿Qué pasa si mi rama avanzó mientras el agente trabajaba?

Si mientras el agente trabajaba en su worktree aislado creaste nuevos commits en tu rama base:

1. Al ejecutar `gz-ia session get <id>`, `gz-ia` trae los archivos creados y modificados por la sesión directamente a tu directorio de trabajo activo como modificaciones no preparadas (*unstaged*).
2. **Cero merge commits o conflictos:** No se realiza `git merge` ni se generan commits automáticos, permitiéndote revisar las diferencias con `git diff` antes de realizar el commit en tu rama base.
3. **Continuidad y Contexto:** Puedes consultar `gz-ia session context <id>` para revisar el historial completo de eventos y archivos mutados antes de integrar.

---

## 10. Glosario Rápido: Toolkit vs Preset vs Driver vs Sesión

| Concepto | Qué es | Dónde vive |
| :--- | :--- | :--- |
| **Toolkit** | La unidad atómica de capacidad de un stack (directivas `AGENTS.md`, reglas `rules/`, skills procedimentales y herramientas MCP). | `.gz-ia/toolkits/<id>` (versionado en el repo), `toolkits/<id>`, `.harness/toolkits/<id>` (local efímero) o `~/.config/gz-ia/tooling/toolkits/<id>` (global) |
| **Preset (Perfil)** | Una composición conveniente de uno o más toolkits (ej. `fullstack = react + postgres`). | `~/.config/gz-ia/tooling/config.json` |
| **Driver** | El adaptador que traduce las intenciones de ejecución al lenguaje específico de un CLI (`agy`, `claude`, `opencode`, `pi-agent`). | Código de `gz-ia` (`internal/features/session/driver.go`) |
| **Sesión** | Una ejecución activa o histórica en un Git Worktree aislado con un agente y toolkits dados. | `.harness/worktrees/<id>` y `.harness/sessions/<id>.json` |

---

## 11. ¿Cómo armo mi propio toolkit?

Puedes inicializar un toolkit modular con un solo comando:
```bash
gz-ia toolkit create mi-stack --desc "Directivas y reglas para mi proyecto"
```
Revisa la guía detallada con un [Ejemplo Completo de Toolkit (React Native + Tailwind)](/guide/profiles#anatomia-de-un-toolkit-modular).

---

## 12. ¿Qué licencia tiene? ¿Puedo usarlo en mi trabajo?

`gz-ia` está licenciado bajo la **Licencia MIT**.
Es software libre y de código abierto sin restricciones: puedes usarlo libremente en proyectos personales, consultoría o en entornos corporativos comerciales.
