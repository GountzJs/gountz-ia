# Estado del Proyecto y Limitaciones Conocidas

`gz-ia` se encuentra en **versión v0.1.0**. El núcleo funcional de orquestación, proyección de toolkits y control en Git es completamente operativo y cuenta con tests automatizados, pero existen limitaciones técnicas y compromisos de diseño que debes conocer antes de usarlo en flujos críticos.

Preferimos documentar estas limitaciones de frente antes de que las descubras por accidente en medio de tu jornada de trabajo.

---

## Limitaciones Técnicas Actuales

### 1. Control de TTY y Señal `SIGTTOU` al Salir del Agente
Al finalizar una sesión de chat con un agente interactivo o en salidas forzadas (`Ctrl+C`), ciertos shells pueden suspender el proceso `gz-ia` si se intenta transferir el control de la terminal al proceso padre mientras este aún no se encuentra en el foreground.
- **Síntoma real:** El shell muestra un mensaje de suspensión:
  ```text
  [1]+ Stopped    gz-ia chat
  ```
- **Workaround:** El comando `reset` no surte efecto porque el proceso no está degradado sino detenido por el kernel (`SIGTTOU`). Para reanudarlo y devolverlo al primer plano, ejecuta en tu terminal:
  ```bash
  fg
  ```
- **Medida aplicada:** Las versiones recientes protegen la llamada a `unix.TIOCSPGRP` ignorando temporalmente `SIGTTOU` y `SIGTTIN` durante la devolución de la terminal interactiva.

### 2. Soporte y Portabilidad en macOS
El arnés compila y ejecuta en entornos Linux y se encuentra en proceso de consolidación para Darwin / macOS. Las discrepancias previas de portabilidad radicaban en llamadas de bajo nivel al control de terminal que dependían de constantes no portables (como `unix.TCGETS` en lugar de `unix.TIOCGETA` o la librería estándar `golang.org/x/term`).

### 3. Actualización en Caliente en Windows
El comando `gz-ia update` intenta descargar paquetes comprimidos `.zip` desde los releases de GitHub y sustituir atómicamente el ejecutable. En sistemas operativos GNU/Linux este reemplazo es nativo vía inodos. En Windows, el kernel bloquea los ejecutables activos en memoria con un error de acceso exclusivo (`Win32 file locking`).
- **Recomendación en Windows:** No ejecutes `gz-ia update` con la sesión activa. Descarga directamente el paquete `.zip` de la nueva versión desde los [Releases de GitHub](https://github.com/GountzJs/gountz-ia/releases) y reemplaza el ejecutable con el terminal cerrado.

### 4. Secretos en el Vault (`vault.json`)
El Vault local almacena variables en texto plano JSON bajo `.harness/vault.json`. Aunque está protegido con permisos POSIX `0600` (accesibles exclusivamente por tu usuario del sistema operativo) e ignorado de Git mediante `.git/info/exclude`:
- **Sin cifrado en reposo:** No implementa cifrado criptográfico simétrico con clave maestra.
- **Inyección completa:** Todas las variables del vault se inyectan en el entorno (`env`) del agente en cada sesión, por lo que comandos como `env` o `printenv` las exponen.
- **Acceso relativo:** Desde el worktree de la sesión (`.harness/worktrees/<id>`), el archivo reside a dos niveles de distancia (`../../vault.json`). No almacenes credenciales bancarias ni de infraestructura de producción sin evaluar tu modelo de seguridad local.

### 5. Frontend Watchers y Dependencias en Git Worktrees (`node_modules`)
Por diseño de Git, los directorios incluidos en `.gitignore` no se copian al crear un nuevo worktree (`.harness/worktrees/<id>`).
- **Colisiones con Watchers (Vite, Webpack, Tailwind):** Los dev servers que vigilan todo el directorio o escaneos con globs tipo `./**/*.{ts,tsx}` detectan las modificaciones del agente dentro de `.harness/worktrees/`, provocando recargas en caliente continuas o clases CSS duplicadas.
  - *Solución:* Añade en `vite.config.ts`:
    ```ts
    server: { watch: { ignored: ['**/.harness/**'] } }
    ```
  - *Solución en Tailwind:* Limita el content a `./src/**/*.{ts,tsx}` en lugar de toda la raíz.
- **Symlinks y la trampa del trailing slash en `.gitignore`:**
  Si creas un symlink a las dependencias de la raíz, recuerda que son **tres niveles**:
  ```bash
  ln -s $(gz-ia session path <id>)/../../../node_modules $(gz-ia session path <id>)/node_modules
  ```
  Si tu `.gitignore` tiene `node_modules/` con barra final, Git no ignorará el enlace simbólico (lo interpreta como un archivo symlink, no un directorio), provocando que `gz-ia session get` intente commitearlo. Usa `node_modules` (sin barra final) en `.gitignore`.

---

## Cómo dar Feedback Concreto

No necesitamos que nos digas simplemente *"está bueno"* o *"no funcionó"*. Para ayudarnos a evolucionar el arnés hacia la versión beta y 1.0, las tres respuestas más valiosas que nos puedes dar son:

```text
1. ¿Qué tarea o flujo intentaste hacer exactamente?
   (Ej. "Quería correr Claude Code en modo supervisado con un toolkit de React Native para refactorizar un hook")

2. ¿En qué comando o paso específico te trabaste o falló?
   (Ej. "Al ejecutar 'gz-ia session get', el agente había dejado un archivo bloqueado y el merge falló con error X")

3. ¿Qué hiciste en vez de usar gz-ia para resolver la tarea?
   (Ej. "Terminé abriendo Claude Code directo en mi directorio raíz y descarté el worktree")
```

Esa última respuesta nos dice con total honestidad si `gz-ia` te ahorró fricción o si te agregó pasos innecesarios.

Para enviar tus respuestas o reportar errores técnicos, abre un Issue en [GitHub Issues](https://github.com/GountzJs/gountz-ia/issues).
