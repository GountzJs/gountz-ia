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
	forgejoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[
			{"name": "tag/1.2.0", "id": "commit1"},
			{"name": "tag/1.1.0", "id": "commit2"}
		]`)
	}))
	defer forgejoServer.Close()

	svc := NewService(Config{
		ForgejoAPIURL:  forgejoServer.URL,
		NexusBaseURL:   "https://nexus.test/repo",
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
	expectedPkg := fmt.Sprintf("gz-ia_1.2.0_linux_%s", runtime.GOARCH)
	if info.PackageName != expectedPkg {
		t.Errorf("packageName esperado %s, obtenido %s", expectedPkg, info.PackageName)
	}
}

func TestCheckLatest_AlreadyUpToDate(t *testing.T) {
	forgejoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[
			{"name": "v1.0.0", "id": "commit1"}
		]`)
	}))
	defer forgejoServer.Close()

	svc := New(Config{
		ForgejoAPIURL:  forgejoServer.URL,
		NexusBaseURL:   "https://nexus.test/repo",
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
	// 1. Error de estado HTTP
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errServer.Close()

	svcErr := NewService(Config{ForgejoAPIURL: errServer.URL})
	if _, err := svcErr.CheckLatest(context.Background()); err == nil {
		t.Error("se esperaba error con HTTP 500 y se obtuvo nil")
	}

	// 2. Respuesta JSON vacía
	emptyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[]`)
	}))
	defer emptyServer.Close()

	svcEmpty := NewService(Config{ForgejoAPIURL: emptyServer.URL})
	if _, err := svcEmpty.CheckLatest(context.Background()); err == nil {
		t.Error("se esperaba error con lista vacía de tags")
	}
}

func TestUpdate_Success(t *testing.T) {
	targetVer := "1.2.0"
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
			Name: fmt.Sprintf("gz-ia_%s_linux_%s/%s", targetVer, goarch, name),
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

	nexusServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/%s/gz-ia_%s_linux_%s.tar.gz", targetVer, targetVer, goarch)
		if r.URL.Path != expectedPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/x-gzip")
		w.Write(buf.Bytes())
	}))
	defer nexusServer.Close()

	tempInstallDir := t.TempDir()

	svc := NewService(Config{
		NexusBaseURL: nexusServer.URL,
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
	goarch := runtime.GOARCH

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: fmt.Sprintf("gz-ia_%s_linux_%s/gz-ia", targetVer, goarch),
		Mode: 0755,
		Size: int64(len("binary-content")),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write([]byte("binary-content"))
	tw.Close()
	gw.Close()

	forgejoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[{"name": "v1.3.0", "id": "c1"}]`)
	}))
	defer forgejoServer.Close()

	nexusServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-gzip")
		w.Write(buf.Bytes())
	}))
	defer nexusServer.Close()

	tempInstallDir := t.TempDir()

	svc := NewService(Config{
		ForgejoAPIURL: forgejoServer.URL,
		NexusBaseURL:  nexusServer.URL,
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

	// 1. Error HTTP de Nexus (404 Not Found)
	nexus404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer nexus404.Close()

	svc := NewService(Config{NexusBaseURL: nexus404.URL})
	if _, err := svc.Update(context.Background(), "9.9.9", tempInstallDir); err == nil {
		t.Error("se esperaba error por 404 de Nexus y se obtuvo nil")
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

	nexusNoBinary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-gzip")
		w.Write(buf.Bytes())
	}))
	defer nexusNoBinary.Close()

	svcNoBin := NewService(Config{NexusBaseURL: nexusNoBinary.URL})
	if _, err := svcNoBin.Update(context.Background(), "1.0.0", tempInstallDir); err == nil {
		t.Error("se esperaba error cuando el tar no contiene gz-ia")
	}
}

func TestCheckLatest_NexusAPI(t *testing.T) {
	nexusSearchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
			"items": [
				{"group": "/gz-ia/0.0.1", "name": "/gz-ia/0.0.1/gz-ia_0.0.1_linux_amd64.tar.gz"},
				{"group": "/gz-ia/0.0.2", "name": "/gz-ia/0.0.2/gz-ia_0.0.2_linux_amd64.tar.gz"},
				{"group": "/other-pkg/1.0.0", "name": "/other-pkg/1.0.0/other.tar.gz"}
			]
		}`)
	}))
	defer nexusSearchServer.Close()

	svc := NewService(Config{
		NexusSearchURL: nexusSearchServer.URL,
		NexusBaseURL:   "https://nexus.test/repo",
		CurrentVersion: "0.0.1",
	})

	info, err := svc.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest con Nexus API falló: %v", err)
	}

	if info.Version != "0.0.2" {
		t.Errorf("versión esperada 0.0.2, obtenida %s", info.Version)
	}
	if !info.IsNewer {
		t.Errorf("esperado isNewer=true, obtenido false")
	}
	expectedPkg := fmt.Sprintf("gz-ia_0.0.2_linux_%s", runtime.GOARCH)
	if info.PackageName != expectedPkg {
		t.Errorf("packageName esperado %s, obtenido %s", expectedPkg, info.PackageName)
	}
	expectedURL := fmt.Sprintf("https://nexus.test/repo/0.0.2/%s.tar.gz", expectedPkg)
	if info.DownloadURL != expectedURL {
		t.Errorf("downloadURL esperado %s, obtenido %s", expectedURL, info.DownloadURL)
	}
}

func TestCheckLatest_NexusFallbackToForgejo(t *testing.T) {
	nexusFailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer nexusFailServer.Close()

	forgejoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[{"name": "v0.0.3", "id": "commit1"}]`)
	}))
	defer forgejoServer.Close()

	svc := NewService(Config{
		NexusSearchURL: nexusFailServer.URL,
		ForgejoAPIURL:  forgejoServer.URL,
		NexusBaseURL:   "https://nexus.test/repo",
		CurrentVersion: "0.0.1",
	})

	info, err := svc.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest con fallback a Forgejo falló: %v", err)
	}
	if info.Version != "0.0.3" {
		t.Errorf("versión esperada 0.0.3 desde Forgejo, obtenida %s", info.Version)
	}
}

func TestCheckLatest_BothFail(t *testing.T) {
	nexusFailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer nexusFailServer.Close()

	forgejoFailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer forgejoFailServer.Close()

	svc := NewService(Config{
		NexusSearchURL: nexusFailServer.URL,
		ForgejoAPIURL:  forgejoFailServer.URL,
	})

	_, err := svc.CheckLatest(context.Background())
	if err == nil {
		t.Fatal("se esperaba error cuando ambas fuentes fallan")
	}
}

func TestEnvVarsConfig(t *testing.T) {
	oldF := os.Getenv("GZ_FORGEJO_API_URL")
	oldN := os.Getenv("GZ_NEXUS_BASE_URL")
	oldNS := os.Getenv("GZ_NEXUS_SEARCH_URL")
	defer func() {
		os.Setenv("GZ_FORGEJO_API_URL", oldF)
		os.Setenv("GZ_NEXUS_BASE_URL", oldN)
		os.Setenv("GZ_NEXUS_SEARCH_URL", oldNS)
	}()

	os.Setenv("GZ_FORGEJO_API_URL", "https://custom-forgejo.org/tags")
	os.Setenv("GZ_NEXUS_BASE_URL", "https://custom-nexus.org/releases/")
	os.Setenv("GZ_NEXUS_SEARCH_URL", "https://custom-nexus.org/service/rest/v1/search")

	svc := NewService().(*updaterService)
	if svc.cfg.ForgejoAPIURL != "https://custom-forgejo.org/tags" {
		t.Errorf("ForgejoAPIURL esperada de env: %s, obtenida: %s", "https://custom-forgejo.org/tags", svc.cfg.ForgejoAPIURL)
	}
	if svc.cfg.NexusBaseURL != "https://custom-nexus.org/releases" {
		t.Errorf("NexusBaseURL esperada sin slash al final: %s, obtenida: %s", "https://custom-nexus.org/releases", svc.cfg.NexusBaseURL)
	}
	if svc.cfg.NexusSearchURL != "https://custom-nexus.org/service/rest/v1/search" {
		t.Errorf("NexusSearchURL esperada de env: %s, obtenida: %s", "https://custom-nexus.org/service/rest/v1/search", svc.cfg.NexusSearchURL)
	}
}
