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

