package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Projector proyecta los assets de un ComposedProfile en el espacio de trabajo de la sesión.
type Projector struct{}

// NewProjector crea una nueva instancia de Projector.
func NewProjector() *Projector {
	return &Projector{}
}

// Project materializa las directivas, skills y configuraciones MCP en el targetDir de la sesión.
func (pr *Projector) Project(ctx context.Context, targetDir string, composed *ComposedProfile, sessionID string, driverID string) error {
	if composed == nil || targetDir == "" {
		return nil
	}

	// 1. Escribir el AGENTS.md raíz generado en caliente
	rootAgentsContent := GenerateRootAgentsMarkdown(composed, sessionID)
	rootAgentsPath := filepath.Join(targetDir, "AGENTS.md")
	if err := os.WriteFile(rootAgentsPath, []byte(rootAgentsContent), 0644); err != nil {
		return fmt.Errorf("error al escribir AGENTS.md maestro en '%s': %w", rootAgentsPath, err)
	}

	// 2. Proyectar archivos de directivas de perfiles individuales (ej. FRONT-AGENTS.md, DATA-AGENTS.md)
	for targetFilename, sourcePath := range composed.AgentsFiles {
		destPath := filepath.Join(targetDir, targetFilename)
		if err := copyFile(sourcePath, destPath); err != nil {
			return fmt.Errorf("error al proyectar directivas '%s' en el workspace: %w", targetFilename, err)
		}
	}

	// 3. Proyectar Skills en .agents/skills/ mediante symlinks (o copia en fallback)
	if len(composed.SkillPaths) > 0 {
		agentsSkillsDir := filepath.Join(targetDir, ".agents", "skills")
		if err := os.MkdirAll(agentsSkillsDir, 0755); err != nil {
			return fmt.Errorf("error al crear directorio de skills '%s': %w", agentsSkillsDir, err)
		}

		for skillName, sourcePath := range composed.SkillPaths {
			targetSkillPath := filepath.Join(agentsSkillsDir, skillName)
			_ = os.Remove(targetSkillPath) // Remover symlink previo si existiera

			// Intentar symlink relativo o absoluto
			err := os.Symlink(sourcePath, targetSkillPath)
			if err != nil {
				// Fallback defensivo a copia recursiva si el SO no soporta symlinks
				_ = copyDir(sourcePath, targetSkillPath)
			}
		}
	}

	// 4. Proyectar configuración de servidores MCP si hay definidos
	if len(composed.MCPServers) > 0 {
		mcpPayload := map[string]any{
			"mcpServers": composed.MCPServers,
		}
		data, err := json.MarshalIndent(mcpPayload, "", "  ")
		if err == nil {
			// Para Antigravity (.agents/mcp_config.json)
			agentsDir := filepath.Join(targetDir, ".agents")
			_ = os.MkdirAll(agentsDir, 0755)
			_ = os.WriteFile(filepath.Join(agentsDir, "mcp_config.json"), data, 0644)

			// Para Claude Code (.mcp.json en la raíz)
			_ = os.WriteFile(filepath.Join(targetDir, ".mcp.json"), data, 0644)
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
