package session

import (
	"errors"
	"strings"
	"testing"
)

func TestDriversList(t *testing.T) {
	drivers := ListDrivers()
	if len(drivers) != 4 {
		t.Fatalf("se esperaban 4 drivers, se obtuvieron %d", len(drivers))
	}

	expectedIDs := []string{"agy", "claude", "opencode", "pi-agent"}
	for i, expectedID := range expectedIDs {
		if drivers[i].ID() != expectedID {
			t.Errorf("driver[%d]: esperado '%s', obtenido '%s'", i, expectedID, drivers[i].ID())
		}
		if drivers[i].BinaryName() != expectedID {
			t.Errorf("driver[%d]: BinaryName esperado '%s', obtenido '%s'", i, expectedID, drivers[i].BinaryName())
		}
		if drivers[i].DisplayName() == "" {
			t.Errorf("driver[%s]: DisplayName no debe estar vacío", expectedID)
		}
		if drivers[i].InstallHint() == "" {
			t.Errorf("driver[%s]: InstallHint no debe estar vacío", expectedID)
		}
	}
}

func TestGetDriver(t *testing.T) {
	tests := []struct {
		id      string
		wantID  string
		wantErr bool
	}{
		{id: "agy", wantID: "agy", wantErr: false},
		{id: "claude", wantID: "claude", wantErr: false},
		{id: "opencode", wantID: "opencode", wantErr: false},
		{id: "pi-agent", wantID: "pi-agent", wantErr: false},
		{id: "", wantID: "agy", wantErr: false}, // default
		{id: "unknown", wantID: "", wantErr: true},
	}

	for _, tc := range tests {
		d, err := GetDriver(tc.id)
		if tc.wantErr {
			if err == nil {
				t.Errorf("GetDriver('%s') debía fallar", tc.id)
			}
		} else {
			if err != nil {
				t.Errorf("GetDriver('%s') error inesperado: %v", tc.id, err)
			}
			if d.ID() != tc.wantID {
				t.Errorf("GetDriver('%s') ID obtenido '%s', esperado '%s'", tc.id, d.ID(), tc.wantID)
			}
		}
	}
}

func TestAgyDriver_BuildArgs(t *testing.T) {
	d, err := GetDriver("agy")
	if err != nil {
		t.Fatalf("no se pudo obtener agy driver: %v", err)
	}

	// WorkingDir vacío
	if _, err := d.BuildArgs(Config{}); err == nil {
		t.Error("esperaba error con WorkingDir vacío")
	}

	// ReadOnly
	args, err := d.BuildArgs(Config{WorkingDir: "/tmp", PermissionLevel: PermissionReadOnly})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(args) != 2 || args[0] != "--mode" || args[1] != "plan" {
		t.Errorf("argumentos readonly inesperados: %v", args)
	}

	// Supervised
	args, err = d.BuildArgs(Config{WorkingDir: "/tmp", PermissionLevel: PermissionSupervised})
	if err != nil || len(args) != 0 {
		t.Errorf("argumentos supervised inesperados: %v", args)
	}

	// Autonomous + Resume + Prompt
	args, err = d.BuildArgs(Config{
		WorkingDir:      "/tmp",
		PermissionLevel: PermissionAutonomous,
		Resume:          true,
		InitialPrompt:   "hola agy",
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	expected := []string{"--dangerously-skip-permissions", "--continue", "-i", "hola agy"}
	if strings.Join(args, " ") != strings.Join(expected, " ") {
		t.Errorf("esperado %v, obtenido %v", expected, args)
	}

	// Permiso desconocido
	if _, err := d.BuildArgs(Config{WorkingDir: "/tmp", PermissionLevel: "invalido"}); err == nil {
		t.Error("esperaba error con permiso inválido")
	}
}

func TestClaudeDriver_BuildArgs(t *testing.T) {
	d, err := GetDriver("claude")
	if err != nil {
		t.Fatalf("no se pudo obtener claude driver: %v", err)
	}

	if _, err := d.BuildArgs(Config{}); err == nil {
		t.Error("esperaba error con WorkingDir vacío")
	}

	// ReadOnly & Supervised
	args, err := d.BuildArgs(Config{WorkingDir: "/tmp", PermissionLevel: PermissionReadOnly})
	if err != nil || len(args) != 0 {
		t.Errorf("argumentos readonly inesperados para claude: %v", args)
	}

	// Autonomous + Resume + Prompt
	args, err = d.BuildArgs(Config{
		WorkingDir:      "/tmp",
		PermissionLevel: PermissionAutonomous,
		Resume:          true,
		InitialPrompt:   "hola claude",
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	expected := []string{"--dangerously-skip-permissions", "--resume", "-p", "hola claude"}
	if strings.Join(args, " ") != strings.Join(expected, " ") {
		t.Errorf("esperado %v, obtenido %v", expected, args)
	}

	if _, err := d.BuildArgs(Config{WorkingDir: "/tmp", PermissionLevel: "desconocido"}); err == nil {
		t.Error("esperaba error con permiso desconocido")
	}
}

func TestOpenCodeDriver_BuildArgs(t *testing.T) {
	d, err := GetDriver("opencode")
	if err != nil {
		t.Fatalf("no se pudo obtener opencode driver: %v", err)
	}

	if _, err := d.BuildArgs(Config{}); err == nil {
		t.Error("esperaba error con WorkingDir vacío")
	}

	// Autonomous + Resume + Prompt
	args, err := d.BuildArgs(Config{
		WorkingDir:      "/tmp",
		PermissionLevel: PermissionAutonomous,
		Resume:          true,
		InitialPrompt:   "analizar código",
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	expected := []string{"--dangerously-skip-permissions", "--continue", "analizar código"}
	if strings.Join(args, " ") != strings.Join(expected, " ") {
		t.Errorf("esperado %v, obtenido %v", expected, args)
	}

	if _, err := d.BuildArgs(Config{WorkingDir: "/tmp", PermissionLevel: "desconocido"}); err == nil {
		t.Error("esperaba error con permiso desconocido")
	}
}

func TestPiAgentDriver_BuildArgs(t *testing.T) {
	d, err := GetDriver("pi-agent")
	if err != nil {
		t.Fatalf("no se pudo obtener pi-agent driver: %v", err)
	}

	if _, err := d.BuildArgs(Config{}); err == nil {
		t.Error("esperaba error con WorkingDir vacío")
	}

	// Autonomous + Resume + Prompt
	args, err := d.BuildArgs(Config{
		WorkingDir:      "/tmp",
		PermissionLevel: PermissionAutonomous,
		Resume:          true,
		InitialPrompt:   "resolver bug",
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	expected := []string{"--dangerously-skip-permissions", "--resume", "-p", "resolver bug"}
	if strings.Join(args, " ") != strings.Join(expected, " ") {
		t.Errorf("esperado %v, obtenido %v", expected, args)
	}

	if _, err := d.BuildArgs(Config{WorkingDir: "/tmp", PermissionLevel: "desconocido"}); err == nil {
		t.Error("esperaba error con permiso desconocido")
	}
}

func TestFirstAvailableDriver_Mocked(t *testing.T) {
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()

	// Caso 1: ninguno disponible
	lookPath = func(file string) (string, error) {
		return "", errors.New("no encontrado")
	}
	if first := FirstAvailableDriver(); first != nil {
		t.Errorf("esperado nil cuando ninguno está en PATH, obtenido %v", first)
	}

	// Caso 2: claude es el primero disponible
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/bin/claude", nil
		}
		return "", errors.New("no encontrado")
	}
	first := FirstAvailableDriver()
	if first == nil || first.ID() != "claude" {
		t.Errorf("esperado claude, obtenido %v", first)
	}

	// Caso 3: pi-agent disponible
	lookPath = func(file string) (string, error) {
		if file == "pi-agent" {
			return "/usr/bin/pi-agent", nil
		}
		return "", errors.New("no encontrado")
	}
	first = FirstAvailableDriver()
	if first == nil || first.ID() != "pi-agent" {
		t.Errorf("esperado pi-agent, obtenido %v", first)
	}
}

func TestBuildArgs_WithUnavailableDriver(t *testing.T) {
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()

	lookPath = func(file string) (string, error) {
		return "", errors.New("binario ausente")
	}

	_, err := BuildArgs(Config{
		Provider:   "claude",
		WorkingDir: "/tmp",
	})
	if err == nil {
		t.Fatal("BuildArgs con driver no disponible debió retornar error")
	}
	if !strings.Contains(err.Error(), "no está disponible en el PATH") {
		t.Errorf("mensaje de error inesperado: %v", err)
	}
	if !strings.Contains(err.Error(), "npm install -g @anthropic-ai/claude-code") {
		t.Errorf("debe incluir InstallHint de claude: %v", err)
	}
}

func TestBuildArgs_WithUnknownProvider(t *testing.T) {
	_, err := BuildArgs(Config{
		Provider:   "no_existe_xyz",
		WorkingDir: "/tmp",
	})
	if err == nil {
		t.Fatal("BuildArgs con proveedor desconocido debió retornar error")
	}
	if !strings.Contains(err.Error(), "proveedor de agente desconocido") {
		t.Errorf("error inesperado: %v", err)
	}
}
