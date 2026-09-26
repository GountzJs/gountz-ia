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
		ForgejoAPIURL:  DefaultForgejoAPIURL,
		NexusBaseURL:   DefaultNexusBaseURL,
		NexusSearchURL: DefaultNexusSearchURL,
		CurrentVersion: version.Version,
	}

	if envForgejo := os.Getenv("GZ_FORGEJO_API_URL"); envForgejo != "" {
		cfg.ForgejoAPIURL = envForgejo
	}
	if envNexus := os.Getenv("GZ_NEXUS_BASE_URL"); envNexus != "" {
		cfg.NexusBaseURL = strings.TrimRight(envNexus, "/")
	}
	if envNexusSearch := os.Getenv("GZ_NEXUS_SEARCH_URL"); envNexusSearch != "" {
		cfg.NexusSearchURL = envNexusSearch
	}

	if len(customCfg) > 0 {
		if customCfg[0].ForgejoAPIURL != "" {
			cfg.ForgejoAPIURL = customCfg[0].ForgejoAPIURL
		}
		if customCfg[0].NexusBaseURL != "" {
			cfg.NexusBaseURL = strings.TrimRight(customCfg[0].NexusBaseURL, "/")
		}
		if customCfg[0].NexusSearchURL != "" {
			cfg.NexusSearchURL = customCfg[0].NexusSearchURL
		} else if customCfg[0].ForgejoAPIURL != "" {
			// Si en customCfg se especifica ForgejoAPIURL sin NexusSearchURL (como en los tests con mock de Forgejo),
			// vaciar NexusSearchURL para evitar llamadas de red a Nexus durante los tests.
			cfg.NexusSearchURL = ""
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

type forgejoTagItem struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type nexusSearchResponse struct {
	Items []nexusSearchItem `json:"items"`
}

type nexusSearchItem struct {
	Group string `json:"group"`
	Name  string `json:"name"`
}

func extractVersionFromNexusItem(group, name string) string {
	var rawVer string
	if strings.HasPrefix(group, "/gz-ia/") {
		rawVer = strings.TrimPrefix(group, "/gz-ia/")
		rawVer = strings.Split(rawVer, "/")[0]
	} else if strings.HasPrefix(group, "gz-ia/") {
		rawVer = strings.TrimPrefix(group, "gz-ia/")
		rawVer = strings.Split(rawVer, "/")[0]
	} else if idx := strings.Index(name, "/gz-ia/"); idx != -1 {
		after := name[idx+len("/gz-ia/"):]
		rawVer = strings.Split(after, "/")[0]
	} else if idx := strings.Index(name, "gz-ia/"); idx != -1 {
		after := name[idx+len("gz-ia/"):]
		rawVer = strings.Split(after, "/")[0]
	}
	return NormalizeVersion(rawVer)
}

func buildReleaseInfo(latestVer, latestTag, currentVersion, nexusBaseURL string) *ReleaseInfo {
	currentVer := NormalizeVersion(currentVersion)
	isNewer := false
	if currentVer == "dev" || currentVer == "" || currentVer == "none" {
		isNewer = true
	} else if CompareVersions(latestVer, currentVer) > 0 {
		isNewer = true
	}

	goarch := runtime.GOARCH
	pkgName := fmt.Sprintf("gz-ia_%s_linux_%s", latestVer, goarch)
	tarballName := fmt.Sprintf("%s.tar.gz", pkgName)
	downloadURL := fmt.Sprintf("%s/%s/%s", strings.TrimRight(nexusBaseURL, "/"), latestVer, tarballName)

	return &ReleaseInfo{
		Tag:         latestTag,
		Version:     latestVer,
		CurrentVer:  currentVersion,
		IsNewer:     isNewer,
		DownloadURL: downloadURL,
		PackageName: pkgName,
	}
}

func (u *updaterService) checkLatestFromNexus(ctx context.Context) (*ReleaseInfo, error) {
	if u.cfg.NexusSearchURL == "" {
		return nil, fmt.Errorf("NexusSearchURL no configurado")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.cfg.NexusSearchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creando petición a Nexus search: %w", err)
	}
	req.Header.Set("User-Agent", "gz-ia-updater")
	req.Header.Set("Accept", "application/json")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error consultando componentes en Nexus (%s): %w", u.cfg.NexusSearchURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("la API de Nexus retornó estado HTTP %d", resp.StatusCode)
	}

	var searchResp nexusSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta de Nexus search: %w", err)
	}

	var latestVer string
	for _, item := range searchResp.Items {
		ver := extractVersionFromNexusItem(item.Group, item.Name)
		if ver == "" {
			continue
		}
		if latestVer == "" || CompareVersions(ver, latestVer) > 0 {
			latestVer = ver
		}
	}

	if latestVer == "" {
		return nil, fmt.Errorf("no se encontraron versiones de gz-ia en Nexus")
	}

	return buildReleaseInfo(latestVer, "v"+latestVer, u.cfg.CurrentVersion, u.cfg.NexusBaseURL), nil
}

func (u *updaterService) checkLatestFromForgejo(ctx context.Context) (*ReleaseInfo, error) {
	if u.cfg.ForgejoAPIURL == "" {
		return nil, fmt.Errorf("ForgejoAPIURL no configurado")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.cfg.ForgejoAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creando petición a Forgejo: %w", err)
	}
	req.Header.Set("User-Agent", "gz-ia-updater")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error consultando tags en Forgejo (%s): %w", u.cfg.ForgejoAPIURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("la API de Forgejo retornó estado HTTP %d", resp.StatusCode)
	}

	var tags []forgejoTagItem
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta de tags de Forgejo: %w", err)
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("no se encontraron tags publicados en el repositorio")
	}

	var latestTag string
	var latestVer string
	for _, t := range tags {
		norm := NormalizeVersion(t.Name)
		if latestVer == "" || CompareVersions(norm, latestVer) > 0 {
			latestVer = norm
			latestTag = t.Name
		}
	}
	if latestTag == "" {
		latestTag = tags[0].Name
		latestVer = NormalizeVersion(latestTag)
	}

	return buildReleaseInfo(latestVer, latestTag, u.cfg.CurrentVersion, u.cfg.NexusBaseURL), nil
}

// CheckLatest consulta la última versión disponible primero en Nexus y luego en Forgejo como fallback.
func (u *updaterService) CheckLatest(ctx context.Context) (*ReleaseInfo, error) {
	var nexusErr error
	if u.cfg.NexusSearchURL != "" {
		info, err := u.checkLatestFromNexus(ctx)
		if err == nil && info != nil {
			return info, nil
		}
		nexusErr = err
	}

	info, forgejoErr := u.checkLatestFromForgejo(ctx)
	if forgejoErr == nil && info != nil {
		return info, nil
	}

	if nexusErr != nil && forgejoErr != nil {
		return nil, fmt.Errorf("error al verificar actualizaciones (Nexus: %v; Forgejo: %v)", nexusErr, forgejoErr)
	}
	if nexusErr != nil {
		return nil, nexusErr
	}
	return nil, forgejoErr
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

	verToInstall := NormalizeVersion(targetVer)
	if verToInstall == "" {
		info, err := u.CheckLatest(ctx)
		if err != nil {
			return nil, fmt.Errorf("error determinando la última versión: %w", err)
		}
		verToInstall = info.Version
	}

	goarch := runtime.GOARCH
	if goarch != "amd64" && goarch != "arm64" {
		return nil, fmt.Errorf("arquitectura '%s' no soportada (solo linux amd64 y arm64)", goarch)
	}

	pkgName := fmt.Sprintf("gz-ia_%s_linux_%s", verToInstall, goarch)
	tarballName := fmt.Sprintf("%s.tar.gz", pkgName)
	downloadURL := fmt.Sprintf("%s/%s/%s", strings.TrimRight(u.cfg.NexusBaseURL, "/"), verToInstall, tarballName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creando petición de descarga: %w", err)
	}
	req.Header.Set("User-Agent", "gz-ia-updater")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error descargando paquete desde Nexus (%s): %w", downloadURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Nexus retornó HTTP %d al solicitar paquete %s. Verifica que la versión esté publicada", resp.StatusCode, tarballName)
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
