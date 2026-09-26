package profile

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// Composer orquesta la unión, validación de colisiones y generación de directivas para múltiples perfiles.
type Composer struct {
	store Store
}

// NewComposer crea una nueva instancia de Composer.
func NewComposer(store Store) *Composer {
	return &Composer{store: store}
}

// Compose fusiona una lista de nombres de perfiles, validando que existan y que no haya colisiones en sus directivas.
func (c *Composer) Compose(profileNames []string) (*ComposedProfile, error) {
	if len(profileNames) == 0 {
		return &ComposedProfile{
			ActiveProfiles: []string{},
			Skills:         []string{},
			SkillPaths:     make(map[string]string),
			AgentsFiles:    make(map[string]string),
			MCPServers:     make(map[string]any),
			Env:            make(map[string]string),
		}, nil
	}

	composed := &ComposedProfile{
		SkillPaths:  make(map[string]string),
		AgentsFiles: make(map[string]string),
		MCPServers:  make(map[string]any),
		Env:         make(map[string]string),
	}

	seenProfiles := make(map[string]bool)
	seenSkills := make(map[string]bool)

	for _, rawName := range profileNames {
		name := strings.TrimSpace(rawName)
		if name == "" || seenProfiles[name] {
			continue
		}
		seenProfiles[name] = true

		p, err := c.store.GetProfile(name)
		if err != nil {
			return nil, err
		}

		composed.ActiveProfiles = append(composed.ActiveProfiles, p.Name)

		// 1. Fusionar skills
		for _, skillName := range p.Skills {
			skillName = strings.TrimSpace(skillName)
			if skillName == "" {
				continue
			}
			if !seenSkills[skillName] {
				seenSkills[skillName] = true
				composed.Skills = append(composed.Skills, skillName)

				// Resolver ruta física del skill
				sk, err := c.store.GetSkill(skillName)
				if err == nil && sk != nil {
					composed.SkillPaths[skillName] = sk.Path
				}
			}
		}

		// 2. Fusionar archivo *-AGENTS.md del perfil
		if p.AgentsPath != "" {
			targetName := p.AgentsFile
			if targetName == "" {
				targetName = filepath.Base(p.AgentsPath)
			}

			// Validar colisión de nombres de archivo de directivas
			if existingPath, conflict := composed.AgentsFiles[targetName]; conflict && existingPath != p.AgentsPath {
				return nil, fmt.Errorf("colisión de directivas agénticas: el archivo '%s' es reclamado por dos perfiles distintos:\n  - %s\n  - %s\nNombra los archivos de forma unívoca (ej. FRONT-AGENTS.md, DATA-AGENTS.md)", targetName, existingPath, p.AgentsPath)
			}

			composed.AgentsFiles[targetName] = p.AgentsPath
		}

		// 3. Fusionar servidores MCP con detección de colisiones (tolera configuraciones idénticas)
		for serverName, srvDef := range p.MCPServers {
			if existingDef, conflict := composed.MCPServers[serverName]; conflict {
				if !reflect.DeepEqual(existingDef, srvDef) {
					return nil, fmt.Errorf("colisión de servidores MCP: el servidor '%s' está definido con configuraciones distintas entre perfiles", serverName)
				}
			}
			composed.MCPServers[serverName] = srvDef
		}

		// 4. Fusionar variables de entorno con detección de colisiones (sin filtrar secretos en el error)
		for k, v := range p.Env {
			if existingVal, conflict := composed.Env[k]; conflict && existingVal != v {
				return nil, fmt.Errorf("colisión de variable de entorno '%s': definida con valores distintos entre perfiles", k)
			}
			composed.Env[k] = v
		}
	}

	sort.Strings(composed.ActiveProfiles)
	sort.Strings(composed.Skills)

	return composed, nil
}

// GenerateRootAgentsMarkdown redacta en caliente el AGENTS.md maestro que indexa los perfiles y skills.
func GenerateRootAgentsMarkdown(composed *ComposedProfile, sessionID string) string {
	var sb strings.Builder

	sb.WriteString("# Directivas de Sesión Agéntica — Gountz IA\n\n")
	sb.WriteString(fmt.Sprintf("> **Sesión ID:** `%s` | **Perfiles Activos:** `%s`\n\n", sessionID, strings.Join(composed.ActiveProfiles, ", ")))

	// 1. Enlaces a perfiles específicos
	if len(composed.AgentsFiles) > 0 {
		sb.WriteString("## 📚 Guías y Directivas de Dominio Activas\n")
		sb.WriteString("Esta sesión combina perfiles especializados. Debes consultar y acatar rigurosamente las reglas definidas en cada guía:\n\n")

		var filesList []string
		for filename := range composed.AgentsFiles {
			filesList = append(filesList, filename)
		}
		sort.Strings(filesList)

		for _, f := range filesList {
			label := strings.TrimSuffix(f, ".md")
			sb.WriteString(fmt.Sprintf("- 🎯 **[%s](./%s)**: Directivas de codificación y arquitectura para este perfil.\n", label, f))
		}
		sb.WriteString("\n")
	}

	// 2. Catálogo de Skills
	if len(composed.Skills) > 0 {
		sb.WriteString("## ⚡ Habilidades Disponibles (.agents/skills)\n")
		sb.WriteString("Las siguientes skills están montadas en `.agents/skills/` y disponibles para ejecución cuando el flujo lo requiera:\n\n")

		for _, skName := range composed.Skills {
			sb.WriteString(fmt.Sprintf("- `%s`: Ubicada en `.agents/skills/%s/SKILL.md`.\n", skName, skName))
		}
		sb.WriteString("\n")
	}

	// 3. Servidores MCP
	if len(composed.MCPServers) > 0 {
		sb.WriteString("## 🔌 Herramientas MCP Conectadas\n")
		sb.WriteString("Servidores externos configurados en esta sesión:\n\n")

		var serverNames []string
		for srv := range composed.MCPServers {
			serverNames = append(serverNames, srv)
		}
		sort.Strings(serverNames)

		for _, srv := range serverNames {
			sb.WriteString(fmt.Sprintf("- **`%s`**: Herramientas integradas vía protocolo MCP.\n", srv))
		}
		sb.WriteString("\n")
	}

	// 4. Buenas prácticas de la sesión
	sb.WriteString("## 🛡️ Reglas Operativas de la Sesión\n")
	sb.WriteString("1. **Aislamiento:** Estás operando dentro de un worktree dedicado. No intentes modificar referencias Git fuera de tu rama ni hacer `git push`.\n")
	sb.WriteString("2. **Auditoría:** Emite eventos de ciclo de vida (`READ`, `PENDING`, `FINISH`) utilizando `gz-ia session log` para que el operador humano pueda monitorear tu avance en tiempo real.\n")
	sb.WriteString("3. **Documentación:** Respeta las pautas arquitectónicas y de estilo especificadas en los archivos de directivas enlazados arriba.\n")

	return sb.String()
}
