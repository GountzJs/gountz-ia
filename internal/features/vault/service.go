package vault

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Service define los casos de uso para gestionar el Vault centralizado y seguro de variables de entorno.
type Service interface {
	LoadMergedEnv(ctx context.Context) (map[string]string, error)
	LoadMergedEnvSlice(ctx context.Context) ([]string, error)
	Get(ctx context.Context, key string) (value string, exists bool, inVault bool, err error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
	ListStatus(ctx context.Context, profileEnvs []string, provider string) ([]EnvStatus, error)
	ValidateRequired(ctx context.Context, requiredKeys []string) ([]string, error)
	VaultPath() string
}

type vaultService struct {
	store Store
}

// NewService crea un nuevo servicio de Vault para el directorio provisto.
func NewService(projectDir string, store ...Store) Service {
	var s Store
	if len(store) > 0 && store[0] != nil {
		s = store[0]
	} else {
		s = NewFileStore(projectDir)
	}
	return &vaultService{store: s}
}

// VaultPath retorna la ruta física donde se aloja el archivo del vault.
func (s *vaultService) VaultPath() string {
	return s.store.Path()
}

// LoadMergedEnv combina las variables de entorno del sistema con las del Vault (las del Vault tienen precedencia).
func (s *vaultService) LoadMergedEnv(ctx context.Context) (map[string]string, error) {
	v, err := s.store.Load()
	if err != nil {
		return nil, err
	}

	merged := make(map[string]string)

	// 1. Cargar variables del sistema operativo actual
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			merged[parts[0]] = parts[1]
		}
	}

	// 2. Sobrescribir con variables explícitas del Vault
	for k, val := range v.Env {
		merged[k] = val
	}

	return merged, nil
}

// LoadMergedEnvSlice retorna la lista formateada en "CLAVE=VALOR" lista para inyectar en cmd.Env.
func (s *vaultService) LoadMergedEnvSlice(ctx context.Context) ([]string, error) {
	merged, err := s.LoadMergedEnv(ctx)
	if err != nil {
		return nil, err
	}

	var result []string
	for k, v := range merged {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(result)
	return result, nil
}

// Get obtiene el valor de una clave, informando si existe y si proviene del Vault o del sistema operativo.
func (s *vaultService) Get(ctx context.Context, key string) (string, bool, bool, error) {
	cleanKey := strings.TrimSpace(key)
	if cleanKey == "" {
		return "", false, false, fmt.Errorf("la clave no puede estar vacía")
	}

	v, err := s.store.Load()
	if err != nil {
		return "", false, false, err
	}

	// 1. Prioridad: Vault local
	if val, ok := v.Env[cleanKey]; ok {
		return val, true, true, nil
	}

	// 2. Fallback: Sistema operativo
	if sysVal, ok := os.LookupEnv(cleanKey); ok {
		return sysVal, true, false, nil
	}

	return "", false, false, nil
}

// Set guarda o actualiza una variable en el vault local del proyecto.
func (s *vaultService) Set(ctx context.Context, key, value string) error {
	cleanKey := strings.TrimSpace(key)
	if cleanKey == "" {
		return fmt.Errorf("el nombre de la variable no puede estar vacío")
	}

	v, err := s.store.Load()
	if err != nil {
		return err
	}

	v.Env[cleanKey] = value
	return s.store.Save(v)
}

// Delete remueve una variable del vault local.
func (s *vaultService) Delete(ctx context.Context, key string) error {
	cleanKey := strings.TrimSpace(key)
	if cleanKey == "" {
		return fmt.Errorf("el nombre de la variable no puede estar vacío")
	}

	v, err := s.store.Load()
	if err != nil {
		return err
	}

	delete(v.Env, cleanKey)
	return s.store.Save(v)
}

// ValidateRequired verifica qué variables de una lista requerida NO existen ni en el vault ni en el sistema.
func (s *vaultService) ValidateRequired(ctx context.Context, requiredKeys []string) ([]string, error) {
	v, err := s.store.Load()
	if err != nil {
		return nil, err
	}

	var missing []string
	seen := make(map[string]bool)

	for _, rawKey := range requiredKeys {
		k := strings.TrimSpace(rawKey)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true

		// Verificar si existe en Vault
		if _, ok := v.Env[k]; ok {
			continue
		}

		// Verificar si existe en variables de entorno del sistema
		if _, ok := os.LookupEnv(k); ok {
			continue
		}

		missing = append(missing, k)
	}

	sort.Strings(missing)
	return missing, nil
}

// ListStatus recopila el inventario de variables en el Vault, sistema y las recomendadas por perfiles/proveedores.
func (s *vaultService) ListStatus(ctx context.Context, profileEnvs []string, provider string) ([]EnvStatus, error) {
	v, err := s.store.Load()
	if err != nil {
		return nil, err
	}

	type metaInfo struct {
		recommendedFor []string
		description    string
	}
	catalog := make(map[string]*metaInfo)

	// 1. Agregar recomendaciones por defecto
	for _, rec := range DefaultRecommendations {
		catalog[rec.Key] = &metaInfo{
			recommendedFor: append([]string{}, rec.RecommendedFor...),
			description:    rec.Description,
		}
	}

	// 2. Agregar recomendaciones de perfiles
	for _, k := range profileEnvs {
		clean := strings.TrimSpace(k)
		if clean == "" {
			continue
		}
		if entry, ok := catalog[clean]; ok {
			entry.recommendedFor = append(entry.recommendedFor, "perfil activo")
		} else {
			catalog[clean] = &metaInfo{
				recommendedFor: []string{"perfil activo"},
				description:    "Variable declarada en perfiles de la sesión",
			}
		}
	}

	// 3. Recopilar todas las claves únicas
	allKeysMap := make(map[string]bool)
	for k := range v.Env {
		allKeysMap[k] = true
	}
	for k := range catalog {
		allKeysMap[k] = true
	}

	var allKeys []string
	for k := range allKeysMap {
		allKeys = append(allKeys, k)
	}
	sort.Strings(allKeys)

	var result []EnvStatus
	for _, k := range allKeys {
		vaultVal, inVault := v.Env[k]
		sysVal, inSys := os.LookupEnv(k)

		exists := inVault || inSys
		effectiveVal := ""
		if inVault {
			effectiveVal = vaultVal
		} else if inSys {
			effectiveVal = sysVal
		}

		var recFor []string
		var desc string
		if info, ok := catalog[k]; ok {
			recFor = info.recommendedFor
			desc = info.description
		}

		masked := ""
		valLen := 0
		if exists {
			masked = MaskSecret(effectiveVal)
			valLen = len(effectiveVal)
		}

		result = append(result, EnvStatus{
			Key:            k,
			Exists:         exists,
			InVault:        inVault,
			InSystem:       inSys,
			MaskedValue:    masked,
			Length:         valLen,
			RecommendedFor: recFor,
			Description:    desc,
		})
	}

	return result, nil
}
