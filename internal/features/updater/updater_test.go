package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.0.0", "1.0.0"},
		{"v1.2.3", "1.2.3"},
		{"tag/2.0.1", "2.0.1"},
		{"  v0.4.5  ", "0.4.5"},
		{"tag/v1.0.0", "1.0.0"},
		{"dev", "dev"},
	}

	for _, tt := range tests {
		got := NormalizeVersion(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeVersion(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1, v2 string
		want   int
	}{
		{"1.1.0", "1.0.0", 1},
		{"1.0.0", "1.1.0", -1},
		{"1.1.0", "1.1.0", 0},
		{"tag/1.1.0", "v1.1.0", 0},
		{"tag/1.2.0", "1.1.9", 1},
		{"0.0.3", "0.0.2", 1},
		{"0.0.2", "0.0.3", -1},
		{"dev", "1.0.0", -1},
		{"1.0.0", "dev", 1},
		{"dev", "dev", 0},
		{"v2.0.0-rc1", "1.9.9", 1},
		{"0.1.0", "0.2.0", -1},
	}

	for _, tt := range tests {
		got := CompareVersions(tt.v1, tt.v2)
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d; want %d", tt.v1, tt.v2, got, tt.want)
		}
	}
}

func TestCheckLatest(t *testing.T) {
	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
			"tag_name": "v1.2.0",
			"name": "Release v1.2.0",
			"assets": [
				{
					"name": "gz-ia_1.2.0_linux_amd64.tar.gz",
					"browser_download_url": "https://github.test/releases/download/v1.2.0/gz-ia_1.2.0_linux_amd64.tar.gz"
				}
			]
		}`)
	}))
	defer ghServer.Close()

	svc := NewService(Config{
		GitHubAPIURL:   ghServer.URL,
		CurrentVersion: "1.1.0",
	})

	info, err := svc.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest falló: %v", err)
	}

	if info.Version != "1.2.0" {
		t.Errorf("versión esperada 1.2.0, obtenida %s", info.Version)
	}
	if !info.IsNewer {
		t.Errorf("esperado isNewer=true, obtenido false")
	}
	expectedPkg := fmt.Sprintf("gz-ia_1.2.0_%s_%s", runtime.GOOS, runtime.GOARCH)
	if info.PackageName != expectedPkg {
		t.Errorf("packageName esperado %s, obtenido %s", expectedPkg, info.PackageName)
	}
}

func TestCheckLatest_AlreadyUpToDate(t *testing.T) {
	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
			"tag_name": "v1.0.0",
			"name": "Release v1.0.0"
		}`)
	}))
	defer ghServer.Close()

	svc := New(Config{
		GitHubAPIURL:   ghServer.URL,
		CurrentVersion: "v1.0.0",
	})

	info, err := svc.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest falló: %v", err)
	}

	if info.IsNewer {
		t.Errorf("esperado isNewer=false cuando las versiones son iguales")
	}
}

func TestCheckLatest_Errors(t *testing.T) {
	// 1. Error HTTP 500
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errServer.Close()

	svcErr := NewService(Config{GitHubAPIURL: errServer.URL})
	if _, err := svcErr.CheckLatest(context.Background()); err == nil {
		t.Error("se esperaba error con HTTP 500 y se obtuvo nil")
	}

	// 2. Respuesta sin tag_name
	emptyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{}`)
	}))
	defer emptyServer.Close()

	svcEmpty := NewService(Config{GitHubAPIURL: emptyServer.URL})
	if _, err := svcEmpty.CheckLatest(context.Background()); err == nil {
		t.Error("se esperaba error con tag_name vacío")
	}
}

func TestUpdate_Success(t *testing.T) {
	targetVer := "1.2.0"
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Creamos un .tar.gz sintético en memoria conteniendo el binario gz-ia y otro archivo ignorado
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	files := map[string]string{
		"gz-ia":     "#!/bin/sh\necho gz-ia-v1.2.0\n",
		"README.md": "# Readme de prueba\n",
	}

	for name, content := range files {
		hdr := &tar.Header{
			Name: fmt.Sprintf("gz-ia_%s_%s_%s/%s", targetVer, goos, goarch, name),
			Mode: 0755,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("error escribiendo tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("error escribiendo contenido tar: %v", err)
		}
	}
	tw.Close()
	gw.Close()

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/v%s/gz-ia_%s_%s_%s.tar.gz", targetVer, targetVer, goos, goarch)
		if r.URL.Path != expectedPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/x-gzip")
		w.Write(buf.Bytes())
	}))
	defer downloadServer.Close()

	tempInstallDir := t.TempDir()

	svc := NewService(Config{
		DownloadBaseURL: downloadServer.URL,
	})

	res, err := svc.Update(context.Background(), targetVer, tempInstallDir)
	if err != nil {
		t.Fatalf("Update falló: %v", err)
	}

	if res.Version != targetVer {
		t.Errorf("versión instalada esperada %s, obtenida %s", targetVer, res.Version)
	}

	if len(res.InstalledBinaries) != 1 || res.InstalledBinaries[0] != "gz-ia" {
		t.Fatalf("binarios instalados inesperados: %v", res.InstalledBinaries)
	}

	installedPath := filepath.Join(tempInstallDir, "gz-ia")
	fi, err := os.Stat(installedPath)
	if err != nil {
		t.Fatalf("binario gz-ia no fue encontrado en destino: %v", err)
	}
	if fi.Mode()&0111 == 0 {
		t.Errorf("el binario instalado gz-ia no tiene permisos de ejecución: %v", fi.Mode())
	}

	// Verificar que README.md NO fue instalado
	if _, err := os.Stat(filepath.Join(tempInstallDir, "README.md")); !os.IsNotExist(err) {
		t.Error("README.md no debería haber sido instalado en el directorio de binarios")
	}
}

func TestUpdate_AutoResolvesLatestVersion(t *testing.T) {
	targetVer := "1.3.0"
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: fmt.Sprintf("gz-ia_%s_%s_%s/gz-ia", targetVer, goos, goarch),
		Mode: 0755,
		Size: int64(len("binary-content")),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write([]byte("binary-content"))
	tw.Close()
	gw.Close()

	var downloadServer *httptest.Server
	downloadServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-gzip")
		w.Write(buf.Bytes())
	}))
	defer downloadServer.Close()

	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"tag_name": "v1.3.0",
			"name": "Release v1.3.0",
			"assets": [
				{
					"name": "gz-ia_1.3.0_%s_%s.tar.gz",
					"browser_download_url": "%s/v1.3.0/gz-ia_1.3.0_%s_%s.tar.gz"
				}
			]
		}`, goos, goarch, downloadServer.URL, goos, goarch)
	}))
	defer ghServer.Close()

	tempInstallDir := t.TempDir()

	svc := NewService(Config{
		GitHubAPIURL:    ghServer.URL,
		DownloadBaseURL: downloadServer.URL,
	})

	// targetVer vacío: debe consultar CheckLatest y usar v1.3.0
	res, err := svc.Update(context.Background(), "", tempInstallDir)
	if err != nil {
		t.Fatalf("Update con targetVer vacío falló: %v", err)
	}

	if res.Version != "1.3.0" {
		t.Errorf("se esperaba versión 1.3.0, obtenida %s", res.Version)
	}
}

func TestUpdate_Errors(t *testing.T) {
	tempInstallDir := t.TempDir()

	// 1. Error HTTP 404
	server404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server404.Close()

	svc := NewService(Config{DownloadBaseURL: server404.URL})
	if _, err := svc.Update(context.Background(), "9.9.9", tempInstallDir); err == nil {
		t.Error("se esperaba error por 404 y se obtuvo nil")
	}

	// 2. Tar sin binario gz-ia
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{
		Name: "otro-archivo.txt",
		Mode: 0644,
		Size: 4,
	}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write([]byte("test"))
	tw.Close()
	gw.Close()

	noBinaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-gzip")
		w.Write(buf.Bytes())
	}))
	defer noBinaryServer.Close()

	svcNoBin := NewService(Config{DownloadBaseURL: noBinaryServer.URL})
	if _, err := svcNoBin.Update(context.Background(), "1.0.0", tempInstallDir); err == nil {
		t.Error("se esperaba error porque el tar no contiene gz-ia")
	}
}

func TestConfig_EnvOverrides(t *testing.T) {
	oldURL := os.Getenv("GZ_GITHUB_API_URL")
	oldRepo := os.Getenv("GZ_GITHUB_REPO")
	oldDL := os.Getenv("GZ_DOWNLOAD_BASE_URL")
	defer func() {
		os.Setenv("GZ_GITHUB_API_URL", oldURL)
		os.Setenv("GZ_GITHUB_REPO", oldRepo)
		os.Setenv("GZ_DOWNLOAD_BASE_URL", oldDL)
	}()

	os.Setenv("GZ_GITHUB_API_URL", "https://api.github.test/custom")
	os.Setenv("GZ_GITHUB_REPO", "myorg/myrepo")
	os.Setenv("GZ_DOWNLOAD_BASE_URL", "https://dl.github.test/releases/")

	svc := NewService().(*updaterService)

	if svc.cfg.GitHubAPIURL != "https://api.github.test/custom" {
		t.Errorf("GitHubAPIURL esperada de env: %s, obtenida: %s", "https://api.github.test/custom", svc.cfg.GitHubAPIURL)
	}
	if svc.cfg.GitHubRepo != "myorg/myrepo" {
		t.Errorf("GitHubRepo esperada de env: %s, obtenida: %s", "myorg/myrepo", svc.cfg.GitHubRepo)
	}
	if svc.cfg.DownloadBaseURL != "https://dl.github.test/releases" {
		t.Errorf("DownloadBaseURL esperada sin slash al final: %s, obtenida: %s", "https://dl.github.test/releases", svc.cfg.DownloadBaseURL)
	}
}
