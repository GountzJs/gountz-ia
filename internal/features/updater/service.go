package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gz-ia/internal/version"
)

// Service define el contrato de casos de uso para verificar e instalar actualizaciones de gz-ia.
type Service interface {
	CheckLatest(ctx context.Context) (*ReleaseInfo, error)
	Update(ctx context.Context, targetVer string, installDir string) (*UpdateResult, error)
}

type updaterService struct {
	cfg        Config
	httpClient *http.Client
}

// NewService crea una nueva instancia de Service con la configuración provista o por defecto.
func NewService(customCfg ...Config) Service {
	cfg := Config{
		GitHubRepo:      DefaultGitHubRepo,
		GitHubAPIURL:    DefaultGitHubAPIURL,
		DownloadBaseURL: DefaultDownloadBaseURL,
		CurrentVersion:  version.Version,
	}

	if envURL := os.Getenv("GZ_GITHUB_API_URL"); envURL != "" {
		cfg.GitHubAPIURL = envURL
	}
	if envRepo := os.Getenv("GZ_GITHUB_REPO"); envRepo != "" {
		cfg.GitHubRepo = envRepo
	}
	if envDownload := os.Getenv("GZ_DOWNLOAD_BASE_URL"); envDownload != "" {
		cfg.DownloadBaseURL = strings.TrimRight(envDownload, "/")
	}

	if len(customCfg) > 0 {
		if customCfg[0].GitHubRepo != "" {
			cfg.GitHubRepo = customCfg[0].GitHubRepo
		}
		if customCfg[0].GitHubAPIURL != "" {
			cfg.GitHubAPIURL = customCfg[0].GitHubAPIURL
		}
		if customCfg[0].DownloadBaseURL != "" {
			cfg.DownloadBaseURL = strings.TrimRight(customCfg[0].DownloadBaseURL, "/")
		}
		if customCfg[0].CurrentVersion != "" {
			cfg.CurrentVersion = customCfg[0].CurrentVersion
		}
	}

	return &updaterService{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// New es un alias conveniente de NewService.
func New(customCfg ...Config) Service {
	return NewService(customCfg...)
}

// NormalizeVersion remueve prefijos 'tag/' o 'v' de un string de versión.
func NormalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "tag/")
	v = strings.TrimPrefix(v, "v")
	return v
}

// CompareVersions compara dos versiones semánticas (v1 vs v2).
// Retorna:
//   1 si v1 > v2
//  -1 si v1 < v2
//   0 si v1 == v2
func CompareVersions(v1, v2 string) int {
	v1Clean := NormalizeVersion(v1)
	v2Clean := NormalizeVersion(v2)

	if v1Clean == v2Clean {
		return 0
	}
	if v1Clean == "dev" {
		return -1
	}
	if v2Clean == "dev" {
		return 1
	}

	p1 := parseSemVerParts(v1Clean)
	p2 := parseSemVerParts(v2Clean)

	for i := 0; i < 3; i++ {
		if p1[i] > p2[i] {
			return 1
		}
		if p1[i] < p2[i] {
			return -1
		}
	}
	return 0
}

func parseSemVerParts(v string) [3]int {
	var parts [3]int
	base := strings.SplitN(v, "-", 2)[0]
	segments := strings.Split(base, ".")
	for i := 0; i < len(segments) && i < 3; i++ {
		val, _ := strconv.Atoi(segments[i])
		parts[i] = val
	}
	return parts
}

type gitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type gitHubRelease struct {
	TagName string        `json:"tag_name"`
	Name    string        `json:"name"`
	Assets  []gitHubAsset `json:"assets"`
}

func buildReleaseInfo(latestVer, latestTag, currentVersion, downloadBaseURL string, assets []gitHubAsset) *ReleaseInfo {
	currentVer := NormalizeVersion(currentVersion)
	isNewer := false
	if currentVer == "dev" || currentVer == "" || currentVer == "none" {
		isNewer = true
	} else if CompareVersions(latestVer, currentVer) > 0 {
		isNewer = true
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH
	pkgName := fmt.Sprintf("gz-ia_%s_%s_%s", latestVer, goos, goarch)
	tarballName := fmt.Sprintf("%s.tar.gz", pkgName)

	downloadURL := fmt.Sprintf("%s/%s/%s", strings.TrimRight(downloadBaseURL, "/"), latestTag, tarballName)
	for _, asset := range assets {
		if asset.Name == tarballName && asset.BrowserDownloadURL != "" {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	return &ReleaseInfo{
		Tag:         latestTag,
		Version:     latestVer,
		CurrentVer:  currentVersion,
		IsNewer:     isNewer,
		DownloadURL: downloadURL,
		PackageName: pkgName,
	}
}

// CheckLatest consulta la última versión disponible en GitHub Releases.
func (u *updaterService) CheckLatest(ctx context.Context) (*ReleaseInfo, error) {
	if u.cfg.GitHubAPIURL == "" {
		return nil, fmt.Errorf("GitHubAPIURL no configurado")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.cfg.GitHubAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creando petición a GitHub Releases: %w", err)
	}
	req.Header.Set("User-Agent", "gz-ia-updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error consultando GitHub Releases (%s): %w", u.cfg.GitHubAPIURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("la API de GitHub retornó estado HTTP %d", resp.StatusCode)
	}

	var rel gitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta de GitHub Releases: %w", err)
	}

	if rel.TagName == "" {
		return nil, fmt.Errorf("no se encontró información de versión (tag_name) en GitHub Releases")
	}

	latestVer := NormalizeVersion(rel.TagName)
	if latestVer == "" {
		return nil, fmt.Errorf("tag_name inválido en GitHub Releases: %s", rel.TagName)
	}

	return buildReleaseInfo(latestVer, rel.TagName, u.cfg.CurrentVersion, u.cfg.DownloadBaseURL, rel.Assets), nil
}

func resolveInstallDir(installDir string) (string, error) {
	if installDir != "" {
		return installDir, nil
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		dir := filepath.Dir(exe)
		if !strings.Contains(dir, "/go-build") && !strings.HasPrefix(dir, os.TempDir()) {
			return dir, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("no se pudo determinar el directorio de usuario HOME: %w", err)
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// Update descarga el paquete de la versión solicitada (o la última si se omite targetVer)
// e instala de forma atómica el binario gz-ia en installDir.
func (u *updaterService) Update(ctx context.Context, targetVer string, installDir string) (*UpdateResult, error) {
	resolvedDir, err := resolveInstallDir(installDir)
	if err != nil {
		return nil, err
	}
	installDir = resolvedDir

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return nil, fmt.Errorf("error asegurando directorio de instalación %s: %w", installDir, err)
	}

	var downloadURL string
	var verToInstall string

	if targetVer == "" {
		info, err := u.CheckLatest(ctx)
		if err != nil {
			return nil, fmt.Errorf("error determinando la última versión: %w", err)
		}
		verToInstall = info.Version
		downloadURL = info.DownloadURL
	} else {
		verToInstall = NormalizeVersion(targetVer)
		tag := "v" + verToInstall
		goos := runtime.GOOS
		goarch := runtime.GOARCH
		tarballName := fmt.Sprintf("gz-ia_%s_%s_%s.tar.gz", verToInstall, goos, goarch)
		downloadURL = fmt.Sprintf("%s/%s/%s", strings.TrimRight(u.cfg.DownloadBaseURL, "/"), tag, tarballName)
	}

	goarch := runtime.GOARCH
	if goarch != "amd64" {
		return nil, fmt.Errorf("arquitectura '%s' no soportada (solo amd64)", goarch)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creando petición de descarga: %w", err)
	}
	req.Header.Set("User-Agent", "gz-ia-updater")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error descargando paquete desde GitHub Releases (%s): %w", downloadURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub retornó HTTP %d al solicitar paquete desde %s. Verifica que la versión esté publicada", resp.StatusCode, downloadURL)
	}

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error abriendo stream gzip: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	var installedBinaries []string

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error leyendo archivo tar: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		baseName := filepath.Base(header.Name)
		if baseName != "gz-ia" {
			continue
		}

		// Reemplazo atómico seguro: escribir en archivo temporal en el mismo directorio y renombrar.
		// Esto evita el error de Linux ETXTBSY (text file busy) cuando el ejecutable está en ejecución.
		tmpFile, err := os.CreateTemp(installDir, fmt.Sprintf(".%s-tmp-*", baseName))
		if err != nil {
			return nil, fmt.Errorf("error creando archivo temporal para %s: %w", baseName, err)
		}
		tmpPath := tmpFile.Name()

		if _, err := io.Copy(tmpFile, tarReader); err != nil {
			tmpFile.Close()
			_ = os.Remove(tmpPath)
			return nil, fmt.Errorf("error escribiendo binario %s: %w", baseName, err)
		}

		if err := tmpFile.Chmod(0755); err != nil {
			tmpFile.Close()
			_ = os.Remove(tmpPath)
			return nil, fmt.Errorf("error configurando permisos ejecutables para %s: %w", baseName, err)
		}
		tmpFile.Close()

		targetPath := filepath.Join(installDir, baseName)
		if err := os.Rename(tmpPath, targetPath); err != nil {
			_ = os.Remove(tmpPath)
			return nil, fmt.Errorf("error moviendo binario temporal a %s: %w", targetPath, err)
		}

		installedBinaries = append(installedBinaries, baseName)
	}

	if len(installedBinaries) == 0 {
		return nil, fmt.Errorf("no se encontró el binario 'gz-ia' en el paquete descargado")
	}

	return &UpdateResult{
		Version:           verToInstall,
		InstalledBinaries: installedBinaries,
		InstallDir:        installDir,
	}, nil
}
