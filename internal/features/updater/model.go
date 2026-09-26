package updater

const (
	// DefaultGitHubRepo es el repositorio oficial de GitHub para gz-ia.
	DefaultGitHubRepo = "GountzJs/gountz-ia"
	// DefaultGitHubAPIURL es la URL por defecto para consultar el último release en GitHub.
	DefaultGitHubAPIURL = "https://api.github.com/repos/GountzJs/gountz-ia/releases/latest"
	// DefaultDownloadBaseURL es la URL base para descargar assets de releases.
	DefaultDownloadBaseURL = "https://github.com/GountzJs/gountz-ia/releases/download"
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
	GitHubRepo      string `json:"github_repo"`
	GitHubAPIURL    string `json:"github_api_url"`
	DownloadBaseURL string `json:"download_base_url"`
	CurrentVersion  string `json:"current_version"`
}

// UpdateResult contiene los detalles y binarios instalados tras una actualización.
type UpdateResult struct {
	Version           string   `json:"version"`
	InstalledBinaries []string `json:"installed_binaries"`
	InstallDir        string   `json:"install_dir"`
}
