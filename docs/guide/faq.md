# Preguntas Frecuentes y Objeciones Reales (FAQ)

Esta sección responde directamente, con honestidad técnica y sin rodeos de marketing, las preguntas y dudas más comunes que surgen al evaluar o utilizar `gz-ia`.

---

## 1. ¿Qué me da que no tenga usando Claude Code o Antigravity directo?

> [!TIP] Respuesta en dos líneas
> **Un flujo uniforme de revisión y control en Git independiente del agente que uses, más toolkits modulares reutilizables (directivas, reglas, skills y MCP) que se proyectan automáticamente en cualquier CLI.**

Si trabajas solo, utilizas un único agente de terminal y estás conforme con que modifique directamente tu directorio de trabajo, **probablemente no necesites `gz-ia`**.

`gz-ia` cobra valor real en dos escenarios:
1. **Alternas entre múltiples agentes:** Quieres usar Antigravity para razonamiento profundo, Claude Code para refactorizaciones rápidas u OpenCode para automatizaciones, sin tener que volver a enseñarle tus linters, convenciones de arquitectura y herramientas MCP a cada herramienta por separado.
2. **Trabajas en equipo:** Quieres que todo el equipo comparta los mismos toolkits de stack (ej. React Native + Tailwind) versionados en el repositorio, garantizando que cualquier agente que levante un desarrollador opere bajo exactamente las mismas reglas.

---

## 2. ¿Es seguro? ¿El agente queda aislado?

**No es un sandbox de sistema operativo.**

`gz-ia` aísla el **árbol de trabajo de Git (Working Tree)** mediante `git worktree`, lo que garantiza que:
- Tu editor no se congela ni se desincroniza mientras el agente edita código en paralelo.
- Tu rama activa y tu `git status` no se alteran.
- Un cambio destructivo en el código del agente queda contenido en una rama efímera (`harness/<id>`) hasta que decidas traerlo.

**Sin embargo:**
- El agente se ejecuta con los **privilegios de tu usuario** en el sistema operativo.
- Si le das permisos autónomos (`--perm autonomous` o `--dangerously-skip-permissions`), el agente puede ejecutar comandos de terminal en tu máquina.
- Los guardrails de Git previenen accidentes cotidianos de sobreescritura, pero no contienen a un proceso malicioso o comandos destructivos intencionales (`rm -rf ~`). Para contención total a nivel de kernel, utiliza contenedores Docker o DevContainers.

---

## 3. ¿Tengo que reinstalar `node_modules` en cada sesión?

Al crear un Git Worktree aislado (`.harness/worktrees/<id>`), Git recrea los archivos rastreados en el repositorio. Los directorios incluidos en `.gitignore` (como `node_modules`, `target/` en Rust o `vendor/`) **no existen inicialmente en el nuevo worktree**.

Si tu agente necesita ejecutar tests o linters que dependen de `node_modules`, tienes estas alternativas prácticas:

1. **Enlace simbólico rápido (Workaround recomendado):**
   Crea un symlink desde el worktree hacia el `node_modules` de tu directorio base:
   ```bash
   ln -s $(gz-ia session path <id>)/../../node_modules $(gz-ia session path <id>)/node_modules
   ```
2. **Gestores con caché centralizada (pnpm / bun):**
   Con `pnpm install` o `bun install` dentro del worktree, la resolución tarda segundos gracias a los enlaces duros compartidos en disco.
3. **Modo directo sin aislamiento:**
   Si estás realizando una consulta rápida o no necesitas aislamiento de rama, puedes correr el agente directamente en tu directorio raíz:
   ```bash
   gz-ia chat -p agy -d .
   ```

---

## 4. ¿Anda en macOS? ¿Y en Windows?

Queremos ser 100% transparentes sobre la compatibilidad de plataformas en la versión actual (Alfa v0.0.3):

| Plataforma | Estado | Detalle |
| :--- | :--- | :--- |
| **Linux (x86_64 / amd64)** | **Soportado** | Plataforma primaria de desarrollo. Todas las características operativas. |
| **macOS (Apple Silicon / Intel)** | **En desarrollo** | Actualmente no compila de forma nativa debido a flags específicas de control de terminal POSIX (`Ctty` / `TIOCSCTTY`) en el paquete de sesiones. Soporte planificado para la v0.1.0. |
| **Windows (amd64)** | **Parcial** | La ejecución de sesiones y la TUI funcionan. Sin embargo, la actualización atómica en caliente (`gz-ia update`) no puede reemplazar el binario en ejecución debido al bloqueo de archivos de Win32. |

---

## 5. ¿Necesito una API Key? ¿Usa mi suscripción?

**`gz-ia` no tiene cuenta propia, servidores de autenticación ni cobra suscripciones.**

`gz-ia` es un arnés local que envuelve los CLIs que ya tienes instalados en tu máquina (`agy`, `claude`, `opencode`, `pi-agent`).
- Consume directamente la autenticación, créditos o API Keys que ya tengas configuradas en cada CLI.
- Si usas Claude Code con suscripción Pro/Team o API Key de Anthropic, `gz-ia` la respeta sin intermediarios.
- Si usas Antigravity con tu cuenta de Google, se ejecuta directamente contra tu sesión local.

---

## 6. ¿Manda algo afuera? ¿Tiene telemetría?

**Cero telemetría. Absolutamente nada sale de tu máquina.**

- No existen pingbacks, analíticas, tracking de uso ni servidores de recolección de datos.
- Toda la metadata de ejecución, trazas y métricas se guardan exclusivamente de forma local dentro de la carpeta `.harness/` de tu propio proyecto.
- El código es 100% auditable y de código abierto bajo licencia MIT.

---

## 7. ¿Dónde quedan mis secretos?

Las variables de entorno y claves de API gestionadas con `gz-ia vault` se almacenan localmente en:
```text
.harness/vault.json
```

**Condiciones de seguridad del Vault:**
- **Permisos estrictos:** Se guarda con permisos POSIX `0600` (lectura y escritura exclusivas para tu usuario del sistema).
- **Protección Git:** Se añade automáticamente a `.gitignore` para prevenir commits accidentales.
- **Formato:** Es un archivo JSON en texto plano en tu disco local. No cuenta con cifrado criptográfico adicional en reposo; no coloques tokens de producción bancaria o infraestructura crítica sin evaluar tu modelo de amenazas local.

---

## 8. ¿Cómo lo saco de mi repositorio?

`gz-ia` está diseñado para no dejar basura permanente en tu proyecto. Para remover todo rastro:

1. **Reconcilia y elimina sesiones activas:**
   ```bash
   gz-ia session prune
   ```
2. **Elimina la carpeta local de arnés:**
   ```bash
   rm -rf .harness/
   ```
3. **Elimina ramas temporales de sesiones si quedó alguna:**
   ```bash
   git branch -D $(git branch --list 'harness/*')
   ```

Una vez eliminada la carpeta `.harness/`, tu repositorio queda exactamente en su estado original sin configuraciones residuales. Consulta la [Guía de Limpieza y Desinstalación](/guide/clean-uninstall) para más detalles.

---

## 9. ¿Qué pasa si mi rama avanzó mientras el agente trabajaba?

Si mientras el agente trabajaba en su worktree aislado tú creaste nuevos commits en tu rama base:

1. Al ejecutar `gz-ia session get <id>` (o `gz-ia session merge <id>`), `gz-ia` realiza un `git merge` estándar de la rama `harness/<id>` sobre tu rama activa.
2. **Si no hay conflictos en las mismas líneas:** Git realiza una fusión limpia automática.
3. **Si existen conflictos:** Git detiene el proceso de fusión informando honestamente los archivos en colisión. `gz-ia` **no sobreescribe ni destruye tu trabajo**. El worktree de la sesión permanece 100% intacto para que puedas resolver los conflictos manualmente con tus herramientas habituales o abortar con `git merge --abort`.

---

## 10. Glosario Rápido: Toolkit vs Preset vs Driver vs Sesión

| Concepto | Qué es | Dónde vive |
| :--- | :--- | :--- |
| **Toolkit** | La unidad atómica de capacidad de un stack (directivas `AGENTS.md`, reglas `rules/`, skills procedimentales y herramientas MCP). | `.harness/toolkits/<id>` o `~/.config/gz-ia/tooling/toolkits/<id>` |
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
