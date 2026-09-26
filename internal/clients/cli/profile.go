package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"gz-ia/internal/clients/tui"
	"gz-ia/internal/features/profile"
	"gz-ia/internal/features/tooling"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var defaultProfileService profile.Service

func getProfileService(workDir string) profile.Service {
	if defaultProfileService != nil {
		return defaultProfileService
	}
	return profile.NewService(profile.WithProjectDir(workDir))
}

func newProfileCmd() *cobra.Command {
	var workDir string

	cmd := &cobra.Command{
		Use:          "profile",
		Short:        "Gestiona perfiles agénticos y catálogo de skills (list, show, path, create, skills)",
		Long:         `Permite listar, inspeccionar, crear perfiles agénticos y explorar el catálogo unificado de skills disponibles en el entorno global y de proyecto.`,
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVarP(&workDir, "dir", "d", "", "Directorio de trabajo (por defecto el directorio actual)")

	// Subcomando list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Lista todos los perfiles agénticos disponibles",
		RunE: func(c *cobra.Command, args []string) error {
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getProfileService(workDir)
			profiles, err := svc.ListProfiles(c.Context())
			if err != nil {
				return err
			}

			toolingSvc := tooling.NewService()
			toolingProfiles, _ := toolingSvc.ListProfiles(c.Context())

			icons := tui.GetIcons()
			if len(profiles) == 0 && len(toolingProfiles) == 0 {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(fmt.Sprintf("%s No hay perfiles agénticos registrados (en ~/.config/gz-ia/profiles, .harness/profiles ni tooling/config.json).", icons.Sparkle)))
				return nil
			}

			header := fmt.Sprintf("%-16s  %-28s  %-8s  %-18s  %-8s", "NOMBRE", "DESCRIPCIÓN", "SKILLS", "AGENTS FILE", "ÁMBITO")
			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSecondary).Render(header))
			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("──────────────────────────────────────────────────────────────────────────────────────────"))

			for _, p := range profiles {
				scopeBadge := "proyecto"
				scopeStyle := lipgloss.NewStyle().Foreground(tui.ColorPrimary)
				if p.Scope == "global" {
					scopeBadge = "global"
					scopeStyle = lipgloss.NewStyle().Foreground(tui.ColorWarning)
				}

				skillsCount := fmt.Sprintf("%d skills", len(p.Skills))
				agentsFile := p.AgentsFile
				if agentsFile == "" {
					agentsFile = "-"
				}

				desc := p.Description
				if len(desc) > 28 {
					desc = desc[:25] + "..."
				}

				row := fmt.Sprintf("%-16s  %-28s  %-8s  %-18s  %-8s",
					lipgloss.NewStyle().Bold(true).Render(p.Name),
					desc,
					skillsCount,
					agentsFile,
					scopeStyle.Render(scopeBadge),
				)
				fmt.Fprintln(c.OutOrStdout(), row)
			}

			for _, tp := range toolingProfiles {
				scopeBadge := "tooling"
				scopeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00D7AF"))
				toolkitsCount := fmt.Sprintf("%d toolkits", len(tp.Toolkits))
				desc := tp.Description
				if len(desc) > 28 {
					desc = desc[:25] + "..."
				}
				row := fmt.Sprintf("%-16s  %-28s  %-8s  %-18s  %-8s",
					lipgloss.NewStyle().Bold(true).Render(tp.Name),
					desc,
					toolkitsCount,
					"-",
					scopeStyle.Render(scopeBadge),
				)
				fmt.Fprintln(c.OutOrStdout(), row)
			}
			return nil
		},
	}

	// Subcomando show
	showCmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Muestra la configuración detallada de un perfil agéntico",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			name := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getProfileService(workDir)
			p, err := svc.GetProfile(c.Context(), name)
			if err != nil {
				toolingSvc := tooling.NewService()
				if tp, tpErr := toolingSvc.GetProfile(c.Context(), name); tpErr == nil && tp != nil {
					icons := tui.GetIcons()
					fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSecondary).Render(fmt.Sprintf("%s Perfil de Tooling: %s", icons.Sparkle, tp.Name)))
					fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("────────────────────────────────────────────────────────────────────────"))
					fmt.Fprintf(c.OutOrStdout(), "  Descripción:  %s\n", tp.Description)
					fmt.Fprintf(c.OutOrStdout(), "  Ámbito:       global (tooling)\n")
					fmt.Fprintf(c.OutOrStdout(), "  Toolkits:     %s\n", strings.Join(tp.Toolkits, ", "))
					return nil
				}
				return err
			}

			icons := tui.GetIcons()
			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSecondary).Render(fmt.Sprintf("%s Perfil Agéntico: %s", icons.Sparkle, p.Name)))
			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("────────────────────────────────────────────────────────────────────────"))
			fmt.Fprintf(c.OutOrStdout(), "  Descripción:  %s\n", p.Description)
			fmt.Fprintf(c.OutOrStdout(), "  Ámbito:       %s\n", p.Scope)
			fmt.Fprintf(c.OutOrStdout(), "  Ruta Base:    %s\n", p.Directory)
			if p.AgentsFile != "" {
				fmt.Fprintf(c.OutOrStdout(), "  Agents File:  %s (%s)\n", p.AgentsFile, p.AgentsPath)
			}
			if len(p.Skills) > 0 {
				fmt.Fprintf(c.OutOrStdout(), "  Skills (%d):   %s\n", len(p.Skills), strings.Join(p.Skills, ", "))
			}
			if len(p.MCPServers) > 0 {
				mcpBytes, _ := json.MarshalIndent(p.MCPServers, "    ", "  ")
				fmt.Fprintf(c.OutOrStdout(), "  MCP Servers:\n%s\n", string(mcpBytes))
			}
			return nil
		},
	}

	// Subcomando path
	pathCmd := &cobra.Command{
		Use:   "path [name]",
		Short: "Imprime la ruta del directorio del perfil o del catálogo",
		RunE: func(c *cobra.Command, args []string) error {
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getProfileService(workDir)

			if len(args) == 0 {
				fmt.Fprintln(c.OutOrStdout(), filepath.Join(svc.ProjectDir(), "profiles"))
				return nil
			}

			p, err := svc.GetProfile(c.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(c.OutOrStdout(), p.Directory)
			return nil
		},
	}

	// Subcomando skills
	skillsCmd := &cobra.Command{
		Use:   "skills",
		Short: "Lista todas las skills disponibles en el catálogo de perfiles",
		RunE: func(c *cobra.Command, args []string) error {
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getProfileService(workDir)
			skills, err := svc.ListSkills(c.Context())
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			if len(skills) == 0 {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(fmt.Sprintf("%s No hay skills en el catálogo (buscar en ~/.config/gz-ia/skills o .harness/skills).", icons.Sparkle)))
				return nil
			}

			header := fmt.Sprintf("%-22s  %-12s  %-8s  %-30s", "SKILL", "CATEGORÍA", "ÁMBITO", "DESCRIPCIÓN")
			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSecondary).Render(header))
			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("──────────────────────────────────────────────────────────────────────────────────────────"))

			for _, sk := range skills {
				scopeBadge := "proyecto"
				scopeStyle := lipgloss.NewStyle().Foreground(tui.ColorPrimary)
				if sk.Scope == "global" {
					scopeBadge = "global"
					scopeStyle = lipgloss.NewStyle().Foreground(tui.ColorWarning)
				}

				category := sk.Category
				if category == "" {
					category = "general"
				}

				desc := sk.Description
				if len(desc) > 30 {
					desc = desc[:27] + "..."
				}

				row := fmt.Sprintf("%-22s  %-12s  %-8s  %-30s",
					lipgloss.NewStyle().Bold(true).Render(sk.Name),
					category,
					scopeStyle.Render(scopeBadge),
					desc,
				)
				fmt.Fprintln(c.OutOrStdout(), row)
			}
			return nil
		},
	}

	// Subcomando create
	var (
		desc       string
		agentsFile string
		global     bool
	)
	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Crea un nuevo perfil agéntico con perfil.json y plantilla opcional",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			name := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getProfileService(workDir)

			p, err := svc.CreateProfile(c.Context(), name, desc, agentsFile, global)
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
				Render(fmt.Sprintf("%s Perfil '%s' creado exitosamente.", icons.Check, p.Name)))
			fmt.Fprintf(c.OutOrStdout(), "  Ubicación: %s\n", p.Directory)
			if p.AgentsFile != "" {
				fmt.Fprintf(c.OutOrStdout(), "  Archivo:   %s\n", p.AgentsPath)
			}
			return nil
		},
	}
	createCmd.Flags().StringVar(&desc, "desc", "", "Descripción del perfil agéntico")
	createCmd.Flags().StringVar(&agentsFile, "agents-file", "", "Nombre del archivo de reglas (ej. FRONT-AGENTS.md)")
	createCmd.Flags().BoolVar(&global, "global", false, "Crear en el ámbito global (~/.config/gz-ia) en lugar de proyecto (.harness)")

	cmd.AddCommand(listCmd)
	cmd.AddCommand(showCmd)
	cmd.AddCommand(pathCmd)
	cmd.AddCommand(skillsCmd)
	cmd.AddCommand(createCmd)

	return cmd
}
