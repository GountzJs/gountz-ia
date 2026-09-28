# Estado del Proyecto y Limitaciones Conocidas

`gz-ia` se encuentra en versión **v0.4.0**. Esta sección documenta las limitaciones técnicas conocidas y consideraciones de configuración para el entorno de trabajo.

---

## Limitaciones Técnicas Actuales

### 1. Control de TTY y Señal `SIGTTOU` al Salir del Agente
Al finalizar una sesión de chat con un agente interactivo o en salidas forzadas (`Ctrl+C`), ciertos shells pueden suspender el proceso `gz-ia` si se intenta transferir el control de la terminal al proceso padre mientras este aún no se encuentra en el foreground.
- **Síntoma:** El shell muestra un mensaje de suspensión:
  ```text
  [1]+ Stopped    gz-ia chat
  ```
- **Workaround:** El proceso no está degradado sino detenido por el kernel (`SIGTTOU`). Para reanudarlo y devolverlo al primer plano, ejecuta en la terminal:
  ```bash
  fg
  ```
- **Mitigación:** La invocación a `unix.TIOCSPGRP` ignora temporalmente `SIGTTOU` y `SIGTTIN` durante la devolución de la terminal interactiva.

### 2. Soporte y Portabilidad en macOS
El arnés compila y ejecuta en entornos Linux y se encuentra en proceso de validación para Darwin / macOS.

### 3. Actualización en Caliente en Windows
El comando `gz-ia update` descarga paquetes comprimidos `.zip` desde los releases de GitHub y sustituye el ejecutable. En Windows, el kernel bloquea los ejecutables activos en memoria (`Win32 file locking`).
- **Procedimiento en Windows:** No ejecutes `gz-ia update` con la sesión activa. Descarga el paquete `.zip` de la nueva versión desde los [Releases de GitHub](https://github.com/GountzJs/gountz-ia/releases) y reemplaza el ejecutable con el terminal cerrado.

### 4. Secretos en el Vault (`vault.json`)
El Vault local almacena variables en texto plano JSON bajo `.harness/vault.json`. Está protegido con permisos POSIX `0600` e ignorado de Git mediante `.git/info/exclude`:
- **Sin cifrado en reposo:** No implementa cifrado criptográfico simétrico con clave maestra.
- **Inyección completa:** Todas las variables del vault se inyectan en el entorno (`env`) del agente en cada sesión, por lo que comandos como `env` o `printenv` las exponen.
- **Acceso relativo:** Desde el worktree de la sesión (`.harness/worktrees/<id>`), el archivo reside a dos niveles de distancia (`../../vault.json`).

### 5. Frontend Watchers, Jest y Dependencias en Git Worktrees (`node_modules`)
Por diseño de Git, los directorios incluidos en `.gitignore` no se copian al crear un nuevo worktree (`.harness/worktrees/<id>`).
- **Colisiones con Watchers (Vite, Webpack, Tailwind):** Los servidores de desarrollo que vigilan todo el directorio detectan las modificaciones dentro de `.harness/worktrees/`, provocando recargas en caliente o procesamiento duplicado de clases CSS.
  - *Configuración en Vite (`vite.config.ts`):*
    ```ts
    server: { watch: { ignored: ['**/.harness/**'] } }
    ```
  - *Configuración en Tailwind:* Limita el content a `./src/**/*.{ts,tsx}` en lugar de la raíz.
- **Jest y Colisión de Módulos Haste:** Al duplicarse el `package.json` en cada worktree, Jest genera un error `Haste module naming collision`.
  - *Configuración en `jest.config.js`:*
    ```js
    modulePathIgnorePatterns: ['<rootDir>/.harness/'],
    ```
- **ESLint Flat Config:** Debe ignorar el árbol de sesiones en `eslint.config.js`:
  ```js
  export default [{ ignores: ['**/.harness/**'] }];
  ```
- **Symlinks hacia dependencias de la raíz:**
  Para enlazar las dependencias desde el worktree:
  ```bash
  ln -s $(gz-ia session path <id>)/../../../node_modules $(gz-ia session path <id>)/node_modules
  ```
  Si `.gitignore` contiene `node_modules/` con barra final, Git interpreta la regla para carpetas reales y no para enlaces simbólicos. Usa `node_modules` (sin barra final) en `.gitignore`.

---

## Reporte de Problemas

Para reportar fallos técnicos o sugerir mejoras en [GitHub Issues](https://github.com/GountzJs/gountz-ia/issues), incluye:
1. **Comando ejecutado y flags:** proveedor, modo de permiso y opciones utilizadas.
2. **Salida y logs:** mensaje de error, código de salida y eventos de la sesión (`gz-ia session logs <id>`).
3. **Entorno del sistema:** sistema operativo, arquitectura (`uname -a`) y versión (`gz-ia version`).
