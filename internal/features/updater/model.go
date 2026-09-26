package updater

const (
	// DefaultForgejoAPIURL es la URL por defecto para consultar tags en Forgejo.
	DefaultForgejoAPIURL = "https://git.pipis.app/api/v1/repos/tomasjs/gz-ia/tags"
	// DefaultNexusBaseURL es la URL base del repositorio Nexus para descargar releases de gz-ia.
	DefaultNexusBaseURL = "https://nexus.pipis.app/repository/go-releases/gz-ia"
	// DefaultNexusSearchURL es la URL por defecto para buscar componentes en Nexus REST API.
	DefaultNexusSearchURL = "https://nexus.pipis.app/service/rest/v1/search?repository=go-releases"
)

// ReleaseInfo contiene la información de la última versión disponible en los repositorios.
type ReleaseInfo struct {
	Tag         string `json:"tag"`
	Version     string `json:"version"`
	CurrentVer  string `json:"current_version"`
	IsNewer     bool   `json:"is_newer"`
	DownloadURL string `json:"download_url"`
	PackageName string `json:"package_name"`
}

// Config define las URLs y parámetros de conexión para el actualizador.
type Config struct {
	ForgejoAPIURL  string `json:"forgejo_api_url"`
	NexusBaseURL   string `json:"nexus_base_url"`
	NexusSearchURL string `json:"nexus_search_url"`
	CurrentVersion string `json:"current_version"`
}

// UpdateResult contiene los detalles y binarios instalados tras una actualización.
type UpdateResult struct {
	Version           string   `json:"version"`
	InstalledBinaries []string `json:"installed_binaries"`
	InstallDir        string   `json:"install_dir"`
}
