package vault

import (
	"context"
	"os"
	"testing"
)

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "(vacía)"},
		{"abc", "**** (3 car.)"},
		{"abcdef", "a***f (6 car.)"},
		{"sk-ant-api03-1234567890abcdef", "sk-...ef (29 caracteres)"},
	}

	for _, tt := range tests {
		got := MaskSecret(tt.input)
		if got != tt.want {
			t.Errorf("MaskSecret(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestFileStore_SaveLoad(t *testing.T) {
	tempDir := t.TempDir()
	store := NewFileStore(tempDir)

	// 1. Cargar archivo inexistente debe retornar Vault vacío sin error
	v, err := store.Load()
	if err != nil {
		t.Fatalf("Load() en almacén inexistente falló: %v", err)
	}
	if len(v.Env) != 0 {
		t.Fatalf("Se esperaba vault vacío, pero contiene: %v", v.Env)
	}

	// 2. Guardar variables
	v.Env["TEST_SECRET"] = "super-secret-123"
	v.Env["API_KEY"] = "key-abc"
	if err := store.Save(v); err != nil {
		t.Fatalf("Save() falló: %v", err)
	}

	// 3. Verificar permisos de archivo 0600
	fi, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("No se pudo obtener información del archivo vault: %v", err)
	}
	perm := fi.Mode().Perm()
	if perm != 0600 {
		t.Errorf("Permisos esperados 0600, obtenidos: %04o", perm)
	}

	// 4. Cargar nuevamente y verificar contenido
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() posterior a Save() falló: %v", err)
	}
	if loaded.Env["TEST_SECRET"] != "super-secret-123" {
		t.Errorf("Valor inesperado para TEST_SECRET: %s", loaded.Env["TEST_SECRET"])
	}
	if loaded.Env["API_KEY"] != "key-abc" {
		t.Errorf("Valor inesperado para API_KEY: %s", loaded.Env["API_KEY"])
	}
}

func TestVaultService_SetGetDelete(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)
	ctx := context.Background()

	// 1. Set
	if err := svc.Set(ctx, "DATABASE_URL", "postgres://user:pass@localhost/db"); err != nil {
		t.Fatalf("Set() falló: %v", err)
	}

	// 2. Get existente en vault
	val, exists, inVault, err := svc.Get(ctx, "DATABASE_URL")
	if err != nil {
		t.Fatalf("Get() falló: %v", err)
	}
	if !exists || !inVault || val != "postgres://user:pass@localhost/db" {
		t.Errorf("Get() inesperado: val=%s, exists=%v, inVault=%v", val, exists, inVault)
	}

	// 3. Get inexistente
	_, exists, _, err = svc.Get(ctx, "NON_EXISTENT_VAR")
	if err != nil {
		t.Fatalf("Get() para inexistente falló: %v", err)
	}
	if exists {
		t.Errorf("Se esperaba exists=false para variable inexistente")
	}

	// 4. Delete
	if err := svc.Delete(ctx, "DATABASE_URL"); err != nil {
		t.Fatalf("Delete() falló: %v", err)
	}
	_, exists, _, _ = svc.Get(ctx, "DATABASE_URL")
	if exists {
		t.Errorf("La variable aún existe tras haber sido eliminada")
	}
}

func TestVaultService_ValidateRequired(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)
	ctx := context.Background()

	// Guardar una en Vault
	_ = svc.Set(ctx, "CONFIGURED_IN_VAULT", "val1")

	// Configurar una en el sistema temporalmente
	os.Setenv("CONFIGURED_IN_SYSTEM", "val2")
	defer os.Unsetenv("CONFIGURED_IN_SYSTEM")

	missing, err := svc.ValidateRequired(ctx, []string{
		"CONFIGURED_IN_VAULT",
		"CONFIGURED_IN_SYSTEM",
		"TOTALLY_MISSING_VAR_1",
		"TOTALLY_MISSING_VAR_2",
	})
	if err != nil {
		t.Fatalf("ValidateRequired() falló: %v", err)
	}

	if len(missing) != 2 {
		t.Fatalf("Se esperaban 2 variables faltantes, obtenidas %d: %v", len(missing), missing)
	}
	if missing[0] != "TOTALLY_MISSING_VAR_1" || missing[1] != "TOTALLY_MISSING_VAR_2" {
		t.Errorf("Faltantes inesperadas: %v", missing)
	}
}

func TestVaultService_LoadMergedEnv(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)
	ctx := context.Background()

	os.Setenv("GZ_TEST_SYSTEM_KEY", "system-value")
	defer os.Unsetenv("GZ_TEST_SYSTEM_KEY")

	// El vault debe tener precedencia
	_ = svc.Set(ctx, "GZ_TEST_SYSTEM_KEY", "vault-override-value")
	_ = svc.Set(ctx, "GZ_TEST_VAULT_ONLY", "vault-only-value")

	merged, err := svc.LoadMergedEnv(ctx)
	if err != nil {
		t.Fatalf("LoadMergedEnv() falló: %v", err)
	}

	if merged["GZ_TEST_SYSTEM_KEY"] != "vault-override-value" {
		t.Errorf("El Vault debió sobreescribir la variable del sistema: obtenida %s", merged["GZ_TEST_SYSTEM_KEY"])
	}
	if merged["GZ_TEST_VAULT_ONLY"] != "vault-only-value" {
		t.Errorf("Variable exclusiva del vault no fue cargada: %s", merged["GZ_TEST_VAULT_ONLY"])
	}

	slice, err := svc.LoadMergedEnvSlice(ctx)
	if err != nil {
		t.Fatalf("LoadMergedEnvSlice() falló: %v", err)
	}
	if len(slice) == 0 {
		t.Errorf("LoadMergedEnvSlice() retornó slice vacío")
	}
}

func TestVaultService_ListStatus(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)
	ctx := context.Background()

	_ = svc.Set(ctx, "ANTHROPIC_API_KEY", "sk-ant-test-secret-value-123456")

	statuses, err := svc.ListStatus(ctx, []string{"CUSTOM_PROFILE_SECRET"}, "claude")
	if err != nil {
		t.Fatalf("ListStatus() falló: %v", err)
	}

	foundAnthropic := false
	foundCustom := false
	for _, st := range statuses {
		if st.Key == "ANTHROPIC_API_KEY" {
			foundAnthropic = true
			if !st.InVault || !st.Exists {
				t.Errorf("ANTHROPIC_API_KEY debió estar en Vault")
			}
			if st.MaskedValue == "" || st.MaskedValue == "sk-ant-test-secret-value-123456" {
				t.Errorf("ANTHROPIC_API_KEY debe estar ofuscada: %s", st.MaskedValue)
			}
		}
		if st.Key == "CUSTOM_PROFILE_SECRET" {
			foundCustom = true
			if len(st.RecommendedFor) == 0 {
				t.Errorf("CUSTOM_PROFILE_SECRET debió tener recomendación de perfil activo")
			}
		}
	}

	if !foundAnthropic {
		t.Errorf("ANTHROPIC_API_KEY no fue encontrada en ListStatus")
	}
	if !foundCustom {
		t.Errorf("CUSTOM_PROFILE_SECRET no fue encontrada en ListStatus")
	}
}
