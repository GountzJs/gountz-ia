# Variables de Entorno y Configuración

`gz-ia` está diseñado para funcionar con *cero configuración* obligatoria, adoptando valores por defecto seguros y estándares. Sin embargo, permite personalizar endpoints de actualización y comportamiento mediante variables de entorno.

---

## Catálogo de Variables de Entorno

| Variable de Entorno | Valor Predeterminado | Propósito y Descripción |
| :--- | :--- | :--- |
| `GZ_NEXUS_SEARCH_URL` | `https://nexus.pipis.app/service/rest/v1/search?repository=go-releases` | Endpoint de la API REST v1 de Sonatype Nexus para buscar y consultar las últimas versiones disponibles de los binarios de `gz-ia`. |
| `GZ_NEXUS_BASE_URL` | `https://nexus.pipis.app/repository/go-releases/gz-ia` | URL base del repositorio de artefactos en Sonatype Nexus desde donde se descarga el tarball comprimido del binario `gz-ia_${VERSION}_${GOOS}_${GOARCH}.tar.gz`. |
| `GZ_FORGEJO_API_URL` | `https://git.pipis.app/api/v1/repos/tomasjs/gz-ia/tags` | URL de la API REST de Forgejo utilizada como fallback automático de resolución de versiones si Sonatype Nexus no responde o está inaccesible. |

---

## Ejemplos de Configuración en Entornos Corporativos

### Configurar Repositorio Privado Nexus

Si tu organización aloja su propio servidor Sonatype Nexus o Artifactory:

```bash
export GZ_NEXUS_SEARCH_URL="https://artifactory.miempresa.com/service/rest/v1/search?repository=releases"
export GZ_NEXUS_BASE_URL="https://artifactory.miempresa.com/repository/releases/gz-ia"

gz-ia update --check
```

### Configurar Servidor Git Privado (Forgejo / Gitea)

Si deseas apuntar el fallback de versiones a otra instancia de Forgejo o Gitea:

```bash
export GZ_FORGEJO_API_URL="https://git.miempresa.com/api/v1/repos/mi-org/gz-ia/tags"

gz-ia update --check
```

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

