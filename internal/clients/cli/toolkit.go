package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gz-ia/internal/clients/tui"
	"gz-ia/internal/features/tooling"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var defaultToolingService tooling.Service

func getToolingService(workDir string) tooling.Service {
	if defaultToolingService != nil {
		return defaultToolingService
	}
	return tooling.NewService("", workDir)
}

func newToolkitCmd() *cobra.Command {
	var workDir string

	cmd := &cobra.Command{
		Use:          "toolkit",
		Aliases:      []string{"toolkits", "profile", "profiles"},
		Short:        "Gestiona toolkits modulares, presets y catálogo de skills (list, show, path, create, skills)",
		Long:         `Permite listar, inspeccionar, inicializar toolkits modulares y explorar el catálogo unificado de skills y presets disponibles en el entorno global y de proyecto.`,
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVarP(&workDir, "dir", "d", "", "Directorio de trabajo (por defecto el directorio actual)")

	// Subcomando list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Lista todos los toolkits y presets agénticos disponibles",
		RunE: func(c *cobra.Command, args []string) error {
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getToolingService(workDir)
			toolkits, err := svc.ListToolkits(c.Context())
			if err != nil {
				return err
			}
			presets, _ := svc.ListPresets(c.Context())

			icons := tui.GetIcons()
			if len(toolkits) == 0 && len(presets) == 0 {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(fmt.Sprintf("%s No hay toolkits ni presets registrados en el proyecto (.gz-ia/toolkits) ni globalmente (~/.config/gz-ia/tooling).", icons.Sparkle)))
				return nil
			}

			if len(toolkits) > 0 {
				header := fmt.Sprintf("%-18s  %-28s  %-8s  %-8s  %-8s", "TOOLKIT", "DESCRIPCIÓN", "SKILLS", "TOOLS", "ÁMBITO")
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSecondary).Render(header))
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("──────────────────────────────────────────────────────────────────────────────────────────"))

				for _, tk := range toolkits {
					scopeBadge := "proyecto"
					scopeStyle := lipgloss.NewStyle().Foreground(tui.ColorPrimary)
					if tk.Scope == "global" {
						scopeBadge = "global"
						scopeStyle = lipgloss.NewStyle().Foreground(tui.ColorWarning)
					}

					skillsCount := fmt.Sprintf("%d skills", len(tk.SkillPaths))
					toolsCount := fmt.Sprintf("%d tools", len(tk.Tools))

					desc := tk.Description
					if desc == "" {
						desc = "-"
					}
					if len(desc) > 28 {
						desc = desc[:25] + "..."
					}

					row := fmt.Sprintf("%-18s  %-28s  %-8s  %-8s  %-8s",
						lipgloss.NewStyle().Bold(true).Render(tk.ID),
						desc,
						skillsCount,
						toolsCount,
						scopeStyle.Render(scopeBadge),
					)
					fmt.Fprintln(c.OutOrStdout(), row)
				}
				fmt.Fprintln(c.OutOrStdout())
			}

			if len(presets) > 0 {
				header := fmt.Sprintf("%-18s  %-38s  %-20s", "PRESET", "DESCRIPCIÓN", "TOOLKITS INCLUIDOS")
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00D7AF")).Render(header))
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("──────────────────────────────────────────────────────────────────────────────────────────"))

				for _, p := range presets {
					desc := p.Description
					if desc == "" {
						desc = "-"
					}
					if len(desc) > 38 {
						desc = desc[:35] + "..."
					}
					tks := strings.Join(p.Toolkits, ", ")
					if len(tks) > 20 {
						tks = tks[:17] + "..."
					}
					row := fmt.Sprintf("%-18s  %-38s  %-20s",
						lipgloss.NewStyle().Bold(true).Render(p.Name),
						desc,
						tks,
					)
					fmt.Fprintln(c.OutOrStdout(), row)
				}
			}

			return nil
		},
	}

	// Subcomando create / init
	var (
		desc   string
		global bool
	)
	createCmd := &cobra.Command{
		Use:     "create <id>",
		Aliases: []string{"init"},
		Short:   "Crea el scaffolding inicial para un nuevo toolkit modular",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getToolingService(workDir)

			req := tooling.CreateToolkitRequest{
				ID:          id,
				Description: desc,
				Global:      global,
			}

			tk, err := svc.CreateToolkit(c.Context(), req)
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
				Render(fmt.Sprintf("%s Toolkit '%s' creado exitosamente.", icons.Check, tk.ID)))
			fmt.Fprintf(c.OutOrStdout(), "  Ubicación:  %s\n", tk.Path)
			fmt.Fprintf(c.OutOrStdout(), "  Ámbito:     %s\n", tk.Scope)
			if tk.AgentsPath != "" {
				fmt.Fprintf(c.OutOrStdout(), "  Directivas: %s\n", tk.AgentsPath)
			}
			return nil
		},
	}
	createCmd.Flags().StringVar(&desc, "desc", "", "Descripción del toolkit modular")
	createCmd.Flags().BoolVar(&global, "global", false, "Crear en el ámbito global (~/.config/gz-ia/tooling) en lugar de proyecto (.harness)")

	// Subcomando show
	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Muestra la configuración detallada de un toolkit o preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getToolingService(workDir)
			icons := tui.GetIcons()

			// 1. Probar como Toolkit
			tk, err := svc.GetToolkit(c.Context(), id)
			if err == nil && tk != nil {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSecondary).Render(fmt.Sprintf("%s Toolkit Modular: %s", icons.Sparkle, tk.ID)))
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("────────────────────────────────────────────────────────────────────────"))
				if tk.Description != "" {
					fmt.Fprintf(c.OutOrStdout(), "  Descripción: %s\n", tk.Description)
				}
				fmt.Fprintf(c.OutOrStdout(), "  Ámbito:      %s\n", tk.Scope)
				fmt.Fprintf(c.OutOrStdout(), "  Ruta Base:   %s\n", tk.Path)
				if tk.AgentsPath != "" {
					fmt.Fprintf(c.OutOrStdout(), "  Directivas:  %s\n", tk.AgentsPath)
				}
				if len(tk.RulesPaths) > 0 {
					var rules []string
					for r := range tk.RulesPaths {
						rules = append(rules, r)
					}
					fmt.Fprintf(c.OutOrStdout(), "  Reglas (%d):   %s\n", len(rules), strings.Join(rules, ", "))
				}
				if len(tk.SkillPaths) > 0 {
					var skills []string
					for s := range tk.SkillPaths {
						skills = append(skills, s)
					}
					fmt.Fprintf(c.OutOrStdout(), "  Skills (%d):   %s\n", len(skills), strings.Join(skills, ", "))
				}
				if len(tk.Tools) > 0 {
					var tools []string
					for _, t := range tk.Tools {
						tools = append(tools, t.Name)
					}
					fmt.Fprintf(c.OutOrStdout(), "  Tools (%d):    %s\n", len(tools), strings.Join(tools, ", "))
				}
				if len(tk.MCPServers) > 0 {
					mcpBytes, _ := json.MarshalIndent(tk.MCPServers, "    ", "  ")
					fmt.Fprintf(c.OutOrStdout(), "  MCP Servers:\n%s\n", string(mcpBytes))
				}
				return nil
			}

			// 2. Probar como Preset
			preset, pErr := svc.GetPreset(c.Context(), id)
			if pErr == nil && preset != nil {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00D7AF")).Render(fmt.Sprintf("%s Preset de Tooling: %s", icons.Sparkle, preset.Name)))
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("────────────────────────────────────────────────────────────────────────"))
				fmt.Fprintf(c.OutOrStdout(), "  Descripción: %s\n", preset.Description)
				fmt.Fprintf(c.OutOrStdout(), "  Toolkits:    %s\n", strings.Join(preset.Toolkits, ", "))
				if len(preset.MCPServers) > 0 {
					mcpBytes, _ := json.MarshalIndent(preset.MCPServers, "    ", "  ")
					fmt.Fprintf(c.OutOrStdout(), "  MCP Servers:\n%s\n", string(mcpBytes))
				}
				return nil
			}

			return fmt.Errorf("toolkit o preset '%s' no encontrado", id)
		},
	}

	// Subcomando path
	pathCmd := &cobra.Command{
		Use:   "path [id]",
		Short: "Imprime la ruta del directorio del toolkit o del catálogo local",
		RunE: func(c *cobra.Command, args []string) error {
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getToolingService(workDir)

			if len(args) == 0 {
				defaultDir := filepath.Join(svc.ProjectDir(), ".gz-ia", "toolkits")
				toolkitsDir := filepath.Join(svc.ProjectDir(), "toolkits")
				harnessDir := filepath.Join(svc.ProjectDir(), ".harness", "toolkits")
				if fi, err := os.Stat(defaultDir); err == nil && fi.IsDir() {
					fmt.Fprintln(c.OutOrStdout(), defaultDir)
				} else if fi, err := os.Stat(toolkitsDir); err == nil && fi.IsDir() {
					fmt.Fprintln(c.OutOrStdout(), toolkitsDir)
				} else if fi, err := os.Stat(harnessDir); err == nil && fi.IsDir() {
					fmt.Fprintln(c.OutOrStdout(), harnessDir)
				} else {
					fmt.Fprintln(c.OutOrStdout(), defaultDir)
				}
				return nil
			}

			tk, err := svc.GetToolkit(c.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(c.OutOrStdout(), tk.Path)
			return nil
		},
	}

	// Subcomando skills
	skillsCmd := &cobra.Command{
		Use:   "skills",
		Short: "Lista todas las skills disponibles en el catálogo de toolkits",
		RunE: func(c *cobra.Command, args []string) error {
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getToolingService(workDir)
			skills, err := svc.ListSkills(c.Context())
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			if len(skills) == 0 {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(fmt.Sprintf("%s No hay skills registradas en ningún toolkit disponible.", icons.Sparkle)))
				return nil
			}

			header := fmt.Sprintf("%-20s  %-12s  %-16s  %-8s  %-26s", "SKILL", "CATEGORÍA", "TOOLKIT", "ÁMBITO", "DESCRIPCIÓN")
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

				tkID := sk.ToolkitID
				if tkID == "" {
					tkID = "-"
				}

				desc := sk.Description
				if len(desc) > 26 {
					desc = desc[:23] + "..."
				}

				row := fmt.Sprintf("%-20s  %-12s  %-16s  %-8s  %-26s",
					lipgloss.NewStyle().Bold(true).Render(sk.Name),
					category,
					tkID,
					scopeStyle.Render(scopeBadge),
					desc,
				)
				fmt.Fprintln(c.OutOrStdout(), row)
			}
			return nil
		},
	}

	cmd.AddCommand(listCmd)
	cmd.AddCommand(createCmd)
	cmd.AddCommand(showCmd)
	cmd.AddCommand(pathCmd)
	cmd.AddCommand(skillsCmd)

	return cmd
}
