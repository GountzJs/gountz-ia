# Estado del Proyecto y Limitaciones Conocidas

`gz-ia` se encuentra actualmente en **Fase Alfa Activa (v0.0.3)**. El núcleo funcional de orquestación, proyección de toolkits y control en Git es completamente operativo y cuenta con tests automatizados, pero existen limitaciones técnicas y compromisos de diseño que debes conocer antes de usarlo en flujos críticos.

Preferimos documentar estas limitaciones de frente antes de que las descubras por accidente en medio de tu jornada de trabajo.

---

## Limitaciones Técnicas Actuales

### 1. Restauración de TTY al cerrar el agente
Al finalizar una sesión de chat con un agente interactivo (especialmente en salidas abruptas con `Ctrl+C` o cancelaciones forzadas), ciertos emuladores de terminal pueden experimentar pérdida temporal del modo *echo* (los caracteres que escribes no se muestran en pantalla).
- **Workaround temporal:** Si tu terminal queda en estado raw tras salir del agente, ejecuta:
  ```bash
  reset
  # o alternativamente:
  stty sane
  ```
- **Solución en progreso:** Estamos perfeccionando el amarre y liberación de los descriptores de terminal mediante llamadas explícitas a `tcsetattr` en la salida de proceso.

### 2. Soporte de macOS en desarrollo
La compilación en plataformas Darwin / macOS falla actualmente en el paquete `session` debido a llamadas directas a campos de control de terminal POSIX (`Ctty` y constantes `syscall`) que difieren entre Linux y macOS.
- **Soporte oficial:** Planificado y en desarrollo activo para la versión `v0.1.0`.

### 3. Actualización en caliente en Windows
El comando `gz-ia update` realiza una sustitución atómica del ejecutable binario en disco (`os.Rename`). En sistemas operativos GNU/Linux este reemplazo es nativo vía inodos aun cuando el proceso esté corriendo. En Windows, el kernel bloquea los ejecutables activos en memoria con un error de acceso exclusivo (`Win32 file locking`).
- **Workaround en Windows:** En Windows, descarga el binario comprimido de la nueva versión directamente desde los [Releases de GitHub](https://github.com/GountzJs/gountz-ia/releases) y sustitúyelo con el proceso cerrado, o reinstala vía PowerShell.

### 4. Secretos en el Vault (`vault.json`)
El Vault local almacena variables en texto plano estructurado JSON bajo `.harness/vault.json`. Aunque está protegido con permisos estrictos POSIX `0600` (solo accesibles por tu usuario) e ignorado por Git, **no implementa cifrado simétrico en reposo con contraseña maestra**.
- **Recomendación:** No almacenes credenciales de producción de infraestructura crítica en el Vault local.

### 5. Dependencias pesadas en Git Worktrees (`node_modules`)
Por diseño de Git, los directorios en `.gitignore` no se copian al aprovisionar un nuevo worktree (`.harness/worktrees/<id>`). En proyectos frontend con `node_modules` de varios gigabytes, recrear dependencias puede causar fricción.
- **Workaround:** Usa symlinks manuales hacia el directorio base o utiliza gestores con almacén compartido global como `pnpm` o `bun`.

---

## Cómo dar Feedback Concreto

No necesitamos que nos digas simplemente *"está bueno"* o *"no me anduvo"*. Para ayudarnos a evolucionar el arnés hacia la versión beta y 1.0, las tres respuestas más valiosas que nos puedes dar son:

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
