package cli

import (
	"fmt"
	"strings"

	"gz-ia/internal/clients/tui"
	"gz-ia/internal/features/vault"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var defaultVaultService vault.Service

func getVaultService(workDir string) vault.Service {
	if defaultVaultService != nil {
		return defaultVaultService
	}
	return vault.NewService(workDir)
}

func newVaultCmd() *cobra.Command {
	var workDir string

	cmd := &cobra.Command{
		Use:          "vault",
		Short:        "Gestiona secretos y variables de entorno centralizadas (list, set, get, delete, path)",
		Long:         `Almacena y suministra variables de entorno y claves de API (.harness/vault.json) con permisos estrictos (0600) para agentes y perfiles sin exponerlas en git.`,
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			// Por defecto ejecuta el subcomando list
			return runVaultList(c, workDir)
		},
	}

	cmd.PersistentFlags().StringVarP(&workDir, "dir", "d", "", "Directorio de trabajo o raíz del proyecto")

	// Subcomando list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Lista variables configuradas en el vault y recomendadas con ofuscación",
		RunE: func(c *cobra.Command, args []string) error {
			return runVaultList(c, workDir)
		},
	}

	// Subcomando set
	var setValue string
	setCmd := &cobra.Command{
		Use:   "set <CLAVE> [VALOR]",
		Short: "Guarda o actualiza una variable en el vault local de forma segura",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(c *cobra.Command, args []string) error {
			targetDir := resolveWorkDir(c.Context(), workDir)
			svc := getVaultService(targetDir)
			key := strings.TrimSpace(args[0])

			val := ""
			if len(args) == 2 {
				val = args[1]
			} else if setValue != "" {
				val = setValue
			} else {
				// Solicitar valor de forma interactiva con máscara para no dejar rastro en historial
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title(fmt.Sprintf("Ingresa el valor secreto para '%s':", key)).
							EchoMode(huh.EchoModePassword).
							Value(&val),
					),
				).WithTheme(tui.CustomHuhTheme())

				if err := form.Run(); err != nil {
					return fmt.Errorf("operación cancelada")
				}
			}

			if err := svc.Set(c.Context(), key, val); err != nil {
				return err
			}

			icons := tui.GetIcons()
			msg := lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
				Render(fmt.Sprintf("%s Variable '%s' guardada exitosamente en el vault (%s)", icons.Check, key, svc.VaultPath()))
			fmt.Fprintln(c.OutOrStdout(), msg)
			return nil
		},
	}
	setCmd.Flags().StringVarP(&setValue, "value", "v", "", "Valor directo (opcional; si se omite se solicita con entrada enmascarada)")

	// Subcomando get
	var reveal bool
	getCmd := &cobra.Command{
		Use:   "get <CLAVE>",
		Short: "Inspecciona el estado de una variable en el vault o sistema",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			targetDir := resolveWorkDir(c.Context(), workDir)
			svc := getVaultService(targetDir)
			key := strings.TrimSpace(args[0])

			val, exists, inVault, err := svc.Get(c.Context(), key)
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			if !exists {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorWarning).
					Render(fmt.Sprintf("%s Variable '%s' no encontrada ni en el vault ni en el sistema.", icons.Cross, key)))
				return nil
			}

			origin := "Sistema Operativo ($ENV)"
			if inVault {
				origin = fmt.Sprintf("Vault del proyecto (%s)", svc.VaultPath())
			}

			fmt.Fprintf(c.OutOrStdout(), "%s Variable: %s\n", icons.Terminal, lipgloss.NewStyle().Bold(true).Render(key))
			fmt.Fprintf(c.OutOrStdout(), "%s Origen:   %s\n", icons.Sparkle, origin)

			if reveal {
				fmt.Fprintf(c.OutOrStdout(), "%s Valor:    %s\n", icons.Bolt, val)
			} else {
				fmt.Fprintf(c.OutOrStdout(), "%s Valor:    %s  (usa --reveal para mostrar valor en texto plano)\n", icons.Lock, vault.MaskSecret(val))
			}
			return nil
		},
	}
	getCmd.Flags().BoolVar(&reveal, "reveal", false, "Muestra el valor secreto completo en texto plano")

	// Subcomando delete
	deleteCmd := &cobra.Command{
		Use:     "delete <CLAVE>",
		Aliases: []string{"rm", "remove"},
		Short:   "Elimina una variable del vault local",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			targetDir := resolveWorkDir(c.Context(), workDir)
			svc := getVaultService(targetDir)
			key := strings.TrimSpace(args[0])

			if err := svc.Delete(c.Context(), key); err != nil {
				return err
			}

			icons := tui.GetIcons()
			msg := lipgloss.NewStyle().Foreground(tui.ColorSuccess).
				Render(fmt.Sprintf("%s Variable '%s' eliminada del vault.", icons.Check, key))
			fmt.Fprintln(c.OutOrStdout(), msg)
			return nil
		},
	}

	// Subcomando path
	pathCmd := &cobra.Command{
		Use:   "path",
		Short: "Imprime la ruta absoluta del archivo vault.json",
		RunE: func(c *cobra.Command, args []string) error {
			targetDir := resolveWorkDir(c.Context(), workDir)
			svc := getVaultService(targetDir)
			fmt.Fprintln(c.OutOrStdout(), svc.VaultPath())
			return nil
		},
	}

	cmd.AddCommand(listCmd)
	cmd.AddCommand(setCmd)
	cmd.AddCommand(getCmd)
	cmd.AddCommand(deleteCmd)
	cmd.AddCommand(pathCmd)

	return cmd
}

func runVaultList(c *cobra.Command, workDir string) error {
	targetDir := resolveWorkDir(c.Context(), workDir)
	svc := getVaultService(targetDir)

	// Extraer variables recomendadas por toolkits y presets si existen
	var recommendedEnvs []string
	toolingSvc := getToolingService(targetDir)
	if toolkits, err := toolingSvc.ListToolkits(c.Context()); err == nil {
		for _, tk := range toolkits {
			for k := range tk.Env {
				recommendedEnvs = append(recommendedEnvs, k)
			}
		}
	}
	if presets, err := toolingSvc.ListPresets(c.Context()); err == nil {
		for _, p := range presets {
			for k := range p.Env {
				recommendedEnvs = append(recommendedEnvs, k)
			}
		}
	}

	statuses, err := svc.ListStatus(c.Context(), recommendedEnvs, "")
	if err != nil {
		return err
	}

	icons := tui.GetIcons()
	title := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorPrimary).
		Render(fmt.Sprintf("%s Vault de Secretos y Variables de Entorno — Gountz IA", icons.Lock))
	pathInfo := lipgloss.NewStyle().Foreground(tui.ColorMuted).
		Render(fmt.Sprintf("  Archivo: %s (Permisos 0600, ignorado por Git)", svc.VaultPath()))

	fmt.Fprintln(c.OutOrStdout(), title)
	fmt.Fprintln(c.OutOrStdout(), pathInfo)
	fmt.Fprintln(c.OutOrStdout())

	header := fmt.Sprintf("%-24s  %-12s  %-24s  %-20s", "VARIABLE", "ESTADO", "VALOR ENMASCARADO", "RECOMENDADA PARA")
	fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSecondary).Render(header))
	fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("─────────────────────────────────────────────────────────────────────────────────────────────"))

	for _, s := range statuses {
		statusBadge := lipgloss.NewStyle().Foreground(tui.ColorMuted).Render("Faltante")
		if s.InVault {
			statusBadge = lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).Render("Vault [✓]")
		} else if s.InSystem {
			statusBadge = lipgloss.NewStyle().Foreground(tui.ColorSecondary).Render("Sistema [$]")
		}

		masked := s.MaskedValue
		if masked == "" {
			masked = lipgloss.NewStyle().Foreground(tui.ColorMuted).Render("(no configurada)")
		}

		rec := strings.Join(s.RecommendedFor, ", ")
		if rec == "" {
			rec = "-"
		}
		if len(rec) > 20 {
			rec = rec[:17] + "..."
		}

		line := fmt.Sprintf("%-24s  %-12s  %-24s  %-20s", s.Key, statusBadge, masked, rec)
		fmt.Fprintln(c.OutOrStdout(), line)
	}

	fmt.Fprintln(c.OutOrStdout())
	tip := lipgloss.NewStyle().Foreground(tui.ColorMuted).
		Render(fmt.Sprintf("Tip: Configura variables con 'gz-ia vault set <VARIABLE>' o usa 'gz-ia vault get <VARIABLE> --reveal'."))
	fmt.Fprintln(c.OutOrStdout(), tip)

	return nil
}
