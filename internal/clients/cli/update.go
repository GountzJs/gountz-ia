package cli

import (
	"context"
	"fmt"
	"gz-ia/internal/clients/tui"
	"gz-ia/internal/features/updater"
	"gz-ia/internal/version"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var defaultUpdaterService updater.Service

func getUpdaterService() updater.Service {
	if defaultUpdaterService != nil {
		return defaultUpdaterService
	}
	return updater.NewService()
}

func newUpdateCmd() *cobra.Command {
	var checkOnly bool
	var force bool
	var targetVersion string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Actualiza gz-ia a la última versión disponible desde Nexus",
		Long: `Comprueba si existen nuevas versiones de gz-ia en los repositorios
de Forgejo y Nexus, y permite la actualización atómica del binario en el sistema.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			icons := tui.GetIcons()
			svc := getUpdaterService()
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			// 1. Si es flag --check, solo consultar e informar estado
			if checkOnly {
				info, err := svc.CheckLatest(ctx)
				if err != nil {
					return fmt.Errorf("error al verificar actualizaciones: %w", err)
				}

				currStyled := lipgloss.NewStyle().Foreground(tui.ColorSecondary).Bold(true).Render("v" + version.Version)
				remoteStyled := lipgloss.NewStyle().Foreground(tui.ColorPrimary).Bold(true).Render("v" + info.Version)

				fmt.Fprintf(cmd.OutOrStdout(), "%s Versión actual: %s\n", icons.Terminal, currStyled)
				fmt.Fprintf(cmd.OutOrStdout(), "%s Versión remota: %s\n", icons.Sparkle, remoteStyled)

				if !info.IsNewer {
					msg := lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
						Render(fmt.Sprintf("%s gz-ia ya se encuentra en la versión más reciente (v%s)", icons.Check, info.Version))
					fmt.Fprintln(cmd.OutOrStdout(), msg)
					return nil
				}

				msg := lipgloss.NewStyle().Foreground(tui.ColorWarning).Bold(true).
					Render(fmt.Sprintf("%s Hay una nueva versión disponible (v%s). Ejecuta 'gz-ia update' para instalarla.", icons.Bolt, info.Version))
				fmt.Fprintln(cmd.OutOrStdout(), msg)
				return nil
			}

			// 2. Si no se especificó --version explícita y no es --force, validar si ya está al día
			var verToInstall string = targetVersion
			if verToInstall == "" {
				info, err := svc.CheckLatest(ctx)
				if err != nil {
					return fmt.Errorf("error al verificar la última versión: %w", err)
				}

				if !info.IsNewer && !force {
					msg := lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
						Render(fmt.Sprintf("%s gz-ia ya se encuentra en la versión más reciente (v%s)", icons.Check, info.Version))
					fmt.Fprintln(cmd.OutOrStdout(), msg)
					return nil
				}
				verToInstall = info.Version
			}

			header := lipgloss.NewStyle().Foreground(tui.ColorPrimary).Bold(true).
				Render(fmt.Sprintf("%s Descargando e instalando gz-ia v%s...", icons.Sync, updater.NormalizeVersion(verToInstall)))
			fmt.Fprintln(cmd.OutOrStdout(), header)

			res, err := svc.Update(ctx, verToInstall, "")
			if err != nil {
				return fmt.Errorf("error durante la actualización de gz-ia: %w", err)
			}

			successMsg := lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
				Render(fmt.Sprintf("%s gz-ia actualizado exitosamente a v%s en %s", icons.Check, res.Version, res.InstallDir))
			fmt.Fprintln(cmd.OutOrStdout(), successMsg)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&checkOnly, "check", "c", false, "Solo comprueba si hay nuevas versiones disponibles sin instalar")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Fuerza la descarga e instalación aunque la versión sea la misma")
	cmd.Flags().StringVar(&targetVersion, "version", "", "Instala una versión específica (ej. 1.2.0)")

	return cmd
}
