# Variables de Entorno y Configuración

`gz-ia` está diseñado para funcionar con *cero configuración* obligatoria, adoptando valores por defecto seguros y estándares. Sin embargo, permite personalizar endpoints de actualización y comportamiento mediante variables de entorno.

---

## Catálogo de Variables de Entorno

| Variable de Entorno | Valor Predeterminado | Propósito y Descripción |
| :--- | :--- | :--- |
| `GITHUB_REPO` | `GountzJs/gountz-ia` | Repositorio de GitHub utilizado por el instalador `install.sh` y el actualizador para consultar releases oficiales. |
| `INSTALL_DIR` | `~/.local/bin` (o `/usr/local/bin`) | Directorio destino para la instalación del binario `gz-ia`. |

---

## Secrets de CI/CD para Pipelines de Automatización (GitHub Actions)

Para los workflows de CI/CD alojados en `.github/workflows/`, se configuran los siguientes secretos en el repositorio (`Settings` $\rightarrow$ `Secrets and variables` $\rightarrow$ `Actions`):

### 1. Despliegue de Documentación (`ci.yml` $\rightarrow$ Cloudflare Pages)

| Secret | Requerido | Descripción |
| :--- | :--- | :--- |
| `CLOUDFLARE_API_TOKEN` | **Sí** | Token de API de Cloudflare con permisos de edición para Cloudflare Pages (`Cloudflare Pages: Edit`). |
| `CLOUDFLARE_ACCOUNT_ID` | **Sí** | Identificador de cuenta de Cloudflare (Account ID). |
| `CLOUDFLARE_PROJECT_NAME` | No | Nombre del proyecto en Cloudflare Pages (por defecto: `gz-ia`). |

### 2. Publicación de Releases Binarios (`release.yml` $\rightarrow$ GitHub Releases)

| Secret / Permiso | Requerido | Descripción |
| :--- | :--- | :--- |
| `GITHUB_TOKEN` | Automático | Provisto por GitHub Actions con permiso `contents: write` para crear el release y adjuntar los binarios de Linux (`amd64`) y Windows (`amd64`). |

---

## Vault Centralizado de Secretos del Proyecto (`.harness/vault.json`)

`gz-ia` cuenta con un sistema de **Vault de Secretos y Variables de Entorno** diseñado para almacenar de forma segura credenciales de API (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, `GITHUB_TOKEN`, etc.) y variables requeridas por perfiles agénticos.

### 🛡️ Principios de Seguridad del Vault

1. **Permisos Estrictos de Sistema de Archivos (`0600`):** El archivo `.harness/vault.json` se crea con permisos `0600` (únicamente lectura y escritura por el usuario propietario).
2. **Aislamiento en Git:** La carpeta `.harness/` y el archivo `vault.json` están excluidos automáticamente en `.gitignore` y en `.git/info/exclude`. **Nunca** son añadidos a commits ni subidos a repositorios remotos.
3. **Escritura Atómica:** Las modificaciones se realizan escribiendo primero en un archivo temporal con permisos `0600` y renombrando atómicamente (`os.Rename`), previniendo corrupciones concurrentes.
4. **Ofuscación por Defecto (Zero-Leak Output):** Todos los comandos de visualización (`gz-ia vault list` o `gz-ia vault get`) enmascaran los secretos (ej. `sk-...ef (29 caracteres)`) a menos que el operador utilice explícitamente el flag `--reveal`.
5. **Entrada Interactiva Segura:** Al ejecutar `gz-ia vault set <VARIABLE>` sin argumentos de valor, la terminal solicita el secreto en modo contraseña oculta (`EchoModePassword`), evitando que el secreto quede grabado en el historial de Bash/Zsh (`~/.bash_history`).

### 🚀 Ciclo de Vida y Detección Automática de Variables Faltantes

Cuando se levanta una sesión agéntica (`gz-ia chat` o `gz-ia start`):

1. **Detección de Requerimientos:** El arnés analiza los perfiles agénticos activos (`composed.Env`) y el agente seleccionado (ej. `claude` requiere `ANTHROPIC_API_KEY`, `opencode` requiere `OPENAI_API_KEY`).
2. **Validación de Existencia:** Comprueba si cada variable requerida existe en el Vault del proyecto o en el entorno del sistema operativo (`$ENV`).
3. **Aviso Oportuno al Usuario:** Si se detectan variables que **no existen** (ausentes), emite una advertencia destacada en `os.Stderr` antes de ejecutar el agente:
   ```text
   ⚠️  Aviso de Entorno: Se detectaron variables no configuradas en el entorno ni en el vault:
      • ANTHROPIC_API_KEY
      • DATABASE_URL
      Puedes configurarlas en el vault seguro con: gz-ia vault set <VARIABLE>
   ```
4. **Inyección en el Proceso:** Carga todas las variables combinadas y las inyecta en el proceso hijo del agente de IA, asegurando que las herramientas y subagentes dispongan de las credenciales necesarias.


