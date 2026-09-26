package profile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileStore_ListProfiles_CollisionDetection(t *testing.T) {
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global")
	projectDir := filepath.Join(tmpDir, "project")

	// Crear perfil "frontend" en global
	globalProfileDir := filepath.Join(globalDir, "profiles", "front_global")
	_ = os.MkdirAll(globalProfileDir, 0755)
	_ = os.WriteFile(filepath.Join(globalProfileDir, "perfil.json"), []byte(`{
		"name": "frontend",
		"description": "Global frontend"
	}`), 0644)

	// Crear perfil con el MISMO nombre "frontend" en proyecto
	projProfileDir := filepath.Join(projectDir, ".harness", "profiles", "front_local")
	_ = os.MkdirAll(projProfileDir, 0755)
	_ = os.WriteFile(filepath.Join(projProfileDir, "perfil.json"), []byte(`{
		"name": "frontend",
		"description": "Project frontend duplicate"
	}`), 0644)

	store := NewFileStore(WithGlobalDir(globalDir), WithProjectDir(projectDir))

	// ListProfiles DEBE romper con error de colisión
	_, err := store.ListProfiles()
	if err == nil {
		t.Fatal("ListProfiles debió fallar por colisión de nombres de perfiles, pero retornó nil")
	}

	if !strings.Contains(err.Error(), "colisión de perfiles detectada") {
		t.Errorf("error esperado sobre colisión, obtenido: %v", err)
	}
	if !strings.Contains(err.Error(), "frontend") {
		t.Errorf("el error debió mencionar el perfil 'frontend': %v", err)
	}
}

func TestFileStore_ListSkills_Discovery(t *testing.T) {
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global")
	projectDir := filepath.Join(tmpDir, "project")

	// Crear skill con SKILL.md y YAML frontmatter
	angularSkillDir := filepath.Join(globalDir, "skills", "front-angular")
	_ = os.MkdirAll(angularSkillDir, 0755)
	_ = os.WriteFile(filepath.Join(angularSkillDir, "SKILL.md"), []byte(`---
name: front-angular
description: "Patrones de diseño y guías para Angular 19"
---
# Angular Guide
`), 0644)

	// Crear skill de data
	postgresSkillDir := filepath.Join(projectDir, ".harness", "skills", "data-postgres")
	_ = os.MkdirAll(postgresSkillDir, 0755)
	_ = os.WriteFile(filepath.Join(postgresSkillDir, "SKILL.md"), []byte(`---
description: 'Consultas avanzadas en PostgreSQL'
---
# Postgres
`), 0644)

	store := NewFileStore(WithGlobalDir(globalDir), WithProjectDir(projectDir))

	skills, err := store.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills falló: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("esperadas 2 skills, obtenidas %d", len(skills))
	}

	angular, err := store.GetSkill("front-angular")
	if err != nil {
		t.Fatalf("GetSkill front-angular falló: %v", err)
	}
	if angular.Category != "front" {
		t.Errorf("Category esperada 'front', obtenida '%s'", angular.Category)
	}
	if !strings.Contains(angular.Description, "Angular 19") {
		t.Errorf("descripción no coincide: %s", angular.Description)
	}

	postgres, err := store.GetSkill("data-postgres")
	if err != nil {
		t.Fatalf("GetSkill data-postgres falló: %v", err)
	}
	if postgres.Category != "data" {
		t.Errorf("Category esperada 'data', obtenida '%s'", postgres.Category)
	}
	if !strings.Contains(postgres.Description, "PostgreSQL") {
		t.Errorf("descripción no coincide: %s", postgres.Description)
	}
}

func TestComposer_Compose_Success(t *testing.T) {
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global")
	projectDir := filepath.Join(tmpDir, "project")

	// 1. Skill front-angular
	_ = os.MkdirAll(filepath.Join(globalDir, "skills", "front-angular"), 0755)
	_ = os.WriteFile(filepath.Join(globalDir, "skills", "front-angular", "SKILL.md"), []byte("# Angular"), 0644)

	// 2. Skill data-postgres
	_ = os.MkdirAll(filepath.Join(globalDir, "skills", "data-postgres"), 0755)
	_ = os.WriteFile(filepath.Join(globalDir, "skills", "data-postgres", "SKILL.md"), []byte("# Postgres"), 0644)

	// 3. Perfil frontend con FRONT-AGENTS.md
	frontDir := filepath.Join(globalDir, "profiles", "frontend")
	_ = os.MkdirAll(frontDir, 0755)
	_ = os.WriteFile(filepath.Join(frontDir, "perfil.json"), []byte(`{
		"name": "frontend",
		"description": "Frontend stack",
		"agents_file": "FRONT-AGENTS.md",
		"skills": ["front-angular"]
	}`), 0644)
	_ = os.WriteFile(filepath.Join(frontDir, "FRONT-AGENTS.md"), []byte("# Reglas Frontend"), 0644)

	// 4. Perfil data con DATA-AGENTS.md y MCP
	dataDir := filepath.Join(globalDir, "profiles", "data")
	_ = os.MkdirAll(dataDir, 0755)
	_ = os.WriteFile(filepath.Join(dataDir, "perfil.json"), []byte(`{
		"name": "data",
		"description": "Database stack",
		"agents_file": "DATA-AGENTS.md",
		"skills": ["data-postgres"],
		"mcp_servers": {
			"pg": {"command": "npx", "args": ["pg-server"]}
		}
	}`), 0644)
	_ = os.WriteFile(filepath.Join(dataDir, "DATA-AGENTS.md"), []byte("# Reglas Data"), 0644)

	store := NewFileStore(WithGlobalDir(globalDir), WithProjectDir(projectDir))
	composer := NewComposer(store)

	composed, err := composer.Compose([]string{"frontend", "data"})
	if err != nil {
		t.Fatalf("Compose falló: %v", err)
	}

	if len(composed.ActiveProfiles) != 2 {
		t.Errorf("ActiveProfiles esperado 2, obtenido: %v", composed.ActiveProfiles)
	}
	if len(composed.Skills) != 2 {
		t.Errorf("Skills esperado 2, obtenido: %v", composed.Skills)
	}
	if len(composed.AgentsFiles) != 2 {
		t.Errorf("AgentsFiles esperado 2 ('FRONT-AGENTS.md', 'DATA-AGENTS.md'), obtenido: %v", composed.AgentsFiles)
	}
	if _, ok := composed.AgentsFiles["FRONT-AGENTS.md"]; !ok {
		t.Error("FRONT-AGENTS.md no encontrado en AgentsFiles")
	}
	if _, ok := composed.AgentsFiles["DATA-AGENTS.md"]; !ok {
		t.Error("DATA-AGENTS.md no encontrado en AgentsFiles")
	}
	if _, ok := composed.MCPServers["pg"]; !ok {
		t.Error("Servidor MCP 'pg' no encontrado en ComposedProfile")
	}

	// 5. Verificar generación de AGENTS.md maestro
	md := GenerateRootAgentsMarkdown(composed, "sess_test_123")
	if !strings.Contains(md, "[FRONT-AGENTS](./FRONT-AGENTS.md)") {
		t.Errorf("AGENTS.md generado no contiene link a FRONT-AGENTS.md:\n%s", md)
	}
	if !strings.Contains(md, "[DATA-AGENTS](./DATA-AGENTS.md)") {
		t.Errorf("AGENTS.md generado no contiene link a DATA-AGENTS.md:\n%s", md)
	}
	if !strings.Contains(md, "front-angular") {
		t.Errorf("AGENTS.md no menciona front-angular:\n%s", md)
	}
}

func TestComposer_AgentsFile_Collision(t *testing.T) {
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global")

	p1Dir := filepath.Join(globalDir, "profiles", "p1")
	_ = os.MkdirAll(p1Dir, 0755)
	_ = os.WriteFile(filepath.Join(p1Dir, "perfil.json"), []byte(`{
		"name": "p1",
		"agents_file": "RULES.md"
	}`), 0644)
	_ = os.WriteFile(filepath.Join(p1Dir, "RULES.md"), []byte("# P1 rules"), 0644)

	p2Dir := filepath.Join(globalDir, "profiles", "p2")
	_ = os.MkdirAll(p2Dir, 0755)
	_ = os.WriteFile(filepath.Join(p2Dir, "perfil.json"), []byte(`{
		"name": "p2",
		"agents_file": "RULES.md"
	}`), 0644)
	_ = os.WriteFile(filepath.Join(p2Dir, "RULES.md"), []byte("# P2 rules"), 0644)

	store := NewFileStore(WithGlobalDir(globalDir), WithProjectDir(tmpDir))
	composer := NewComposer(store)

	_, err := composer.Compose([]string{"p1", "p2"})
	if err == nil {
		t.Fatal("se esperaba error por colisión de archivo de directivas 'RULES.md', pero retornó nil")
	}
	if !strings.Contains(err.Error(), "colisión de directivas agénticas") {
		t.Errorf("mensaje de error inesperado: %v", err)
	}
}

func TestComposer_MCPServer_Collision(t *testing.T) {
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global")

	p1Dir := filepath.Join(globalDir, "profiles", "p1")
	_ = os.MkdirAll(p1Dir, 0755)
	_ = os.WriteFile(filepath.Join(p1Dir, "perfil.json"), []byte(`{
		"name": "p1",
		"mcp_servers": {
			"db": {"command": "npx", "args": ["pg"]}
		}
	}`), 0644)

	p2Dir := filepath.Join(globalDir, "profiles", "p2")
	_ = os.MkdirAll(p2Dir, 0755)
	_ = os.WriteFile(filepath.Join(p2Dir, "perfil.json"), []byte(`{
		"name": "p2",
		"mcp_servers": {
			"db": {"command": "docker", "args": ["run"]}
		}
	}`), 0644)

	store := NewFileStore(WithGlobalDir(globalDir), WithProjectDir(tmpDir))
	composer := NewComposer(store)

	_, err := composer.Compose([]string{"p1", "p2"})
	if err == nil {
		t.Fatal("se esperaba error por colisión de servidor MCP 'db', pero retornó nil")
	}
	if !strings.Contains(err.Error(), "colisión de servidores MCP") {
		t.Errorf("mensaje de error inesperado: %v", err)
	}
}

func TestComposer_Env_Collision(t *testing.T) {
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global")

	p1Dir := filepath.Join(globalDir, "profiles", "p1")
	_ = os.MkdirAll(p1Dir, 0755)
	_ = os.WriteFile(filepath.Join(p1Dir, "perfil.json"), []byte(`{
		"name": "p1",
		"env": {"API_URL": "https://api.v1.com"}
	}`), 0644)

	p2Dir := filepath.Join(globalDir, "profiles", "p2")
	_ = os.MkdirAll(p2Dir, 0755)
	_ = os.WriteFile(filepath.Join(p2Dir, "perfil.json"), []byte(`{
		"name": "p2",
		"env": {"API_URL": "https://api.v2.com"}
	}`), 0644)

	store := NewFileStore(WithGlobalDir(globalDir), WithProjectDir(tmpDir))
	composer := NewComposer(store)

	_, err := composer.Compose([]string{"p1", "p2"})
	if err == nil {
		t.Fatal("se esperaba error por colisión de variable de entorno 'API_URL', pero retornó nil")
	}
	if !strings.Contains(err.Error(), "colisión de variable de entorno") {
		t.Errorf("mensaje de error inesperado: %v", err)
	}
}

func TestProjector_Project(t *testing.T) {
	tmpDir := t.TempDir()
	sourceSkillDir := filepath.Join(tmpDir, "source_skills", "front-react")
	_ = os.MkdirAll(sourceSkillDir, 0755)
	_ = os.WriteFile(filepath.Join(sourceSkillDir, "SKILL.md"), []byte("# React skill"), 0644)

	frontAgentsSrc := filepath.Join(tmpDir, "src_FRONT-AGENTS.md")
	_ = os.WriteFile(frontAgentsSrc, []byte("# Directivas React"), 0644)

	worktreeTarget := filepath.Join(tmpDir, "worktree_session")
	_ = os.MkdirAll(worktreeTarget, 0755)

	composed := &ComposedProfile{
		ActiveProfiles: []string{"react-stack"},
		Skills:         []string{"front-react"},
		SkillPaths: map[string]string{
			"front-react": sourceSkillDir,
		},
		AgentsFiles: map[string]string{
			"FRONT-AGENTS.md": frontAgentsSrc,
		},
		MCPServers: map[string]any{
			"test-mcp": map[string]any{"command": "echo"},
		},
		Env: map[string]string{},
	}

	projector := NewProjector()
	ctx := context.Background()

	err := projector.Project(ctx, worktreeTarget, composed, "sess_proj", "agy")
	if err != nil {
		t.Fatalf("Project falló: %v", err)
	}

	// 1. Verificar AGENTS.md maestro
	rootMD := filepath.Join(worktreeTarget, "AGENTS.md")
	data, err := os.ReadFile(rootMD)
	if err != nil {
		t.Fatalf("no se pudo leer AGENTS.md en worktree: %v", err)
	}
	if !strings.Contains(string(data), "FRONT-AGENTS.md") {
		t.Errorf("AGENTS.md no contiene referencia a FRONT-AGENTS.md: %s", string(data))
	}

	// 2. Verificar que FRONT-AGENTS.md fue proyectado en el worktree
	projectedFront := filepath.Join(worktreeTarget, "FRONT-AGENTS.md")
	pData, err := os.ReadFile(projectedFront)
	if err != nil {
		t.Fatalf("FRONT-AGENTS.md no fue proyectado: %v", err)
	}
	if !strings.Contains(string(pData), "Directivas React") {
		t.Errorf("contenido inesperado en FRONT-AGENTS.md: %s", string(pData))
	}

	// 3. Verificar skill montado en .agents/skills/front-react
	projectedSkillMD := filepath.Join(worktreeTarget, ".agents", "skills", "front-react", "SKILL.md")
	sData, err := os.ReadFile(projectedSkillMD)
	if err != nil {
		t.Fatalf("skill no accesible en .agents/skills/front-react: %v", err)
	}
	if !strings.Contains(string(sData), "React skill") {
		t.Errorf("contenido inesperado en skill: %s", string(sData))
	}

	// 4. Verificar .agents/mcp_config.json
	mcpPath := filepath.Join(worktreeTarget, ".agents", "mcp_config.json")
	if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
		t.Error(".agents/mcp_config.json no fue creado")
	}
}

func TestService_CreateProfile(t *testing.T) {
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global")
	projectDir := filepath.Join(tmpDir, "project")

	svc := NewService(WithGlobalDir(globalDir), WithProjectDir(projectDir))
	ctx := context.Background()

	// Crear perfil global
	p, err := svc.CreateProfile(ctx, "devops", "Perfil de infraestructura", "DEVOPS-AGENTS.md", true)
	if err != nil {
		t.Fatalf("CreateProfile falló: %v", err)
	}

	if p.Name != "devops" {
		t.Errorf("nombre esperado 'devops', obtenido '%s'", p.Name)
	}
	if p.AgentsFile != "DEVOPS-AGENTS.md" {
		t.Errorf("agentsFile esperado 'DEVOPS-AGENTS.md', obtenido '%s'", p.AgentsFile)
	}

	// Verificar que el perfil puede ser listado y encontrado
	retrieved, err := svc.GetProfile(ctx, "devops")
	if err != nil {
		t.Fatalf("GetProfile devops falló: %v", err)
	}
	if retrieved.Description != "Perfil de infraestructura" {
		t.Errorf("descripción no coincide: %s", retrieved.Description)
	}

	// Intentar crear nuevamente con el mismo nombre debe fallar
	_, err = svc.CreateProfile(ctx, "devops", "Duplicado", "", true)
	if err == nil {
		t.Fatal("se esperaba error al crear perfil duplicado, pero retornó nil")
	}
}
