package cli

import (
	"fmt"
	"gz-ia/internal/clients/tui"
	"gz-ia/internal/features/session"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var defaultSessionRunner session.Runner = &session.OSRunner{}

func newChatCmd() *cobra.Command {
	var (
		prompt   string
		workDir  string
		perm     string
		provider string
		profiles []string
	)

	defaultProvider := "agy"
	if first := session.FirstAvailableDriver(); first != nil {
		defaultProvider = first.ID()
	}

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Inicia un nuevo chat interactivo con el agente (agy, claude, opencode, pi-agent)",
		Long:  `Lanza una sesión de chat directa con un agente de terminal (agy, claude, opencode, pi-agent) configurando el nivel de permisos (readonly, supervised, autonomous) y perfiles agénticos (-P, --profile).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			drv, err := session.GetDriver(provider)
			if err != nil {
				return fmt.Errorf("proveedor inválido '%s': %w", provider, err)
			}
			if !drv.IsAvailable() {
				return fmt.Errorf("el agente '%s' (%s) no está instalado en tu sistema.\nInstalación: %s", drv.DisplayName(), drv.BinaryName(), drv.InstallHint())
			}

			svc := getSessionService(workDir)
			req := session.StartChatRequest{
				Provider:        drv.ID(),
				WorkingDir:      workDir,
				InitialPrompt:   prompt,
				PermissionLevel: session.PermissionLevel(perm),
				BinaryPath:      drv.BinaryName(),
				Profiles:        profiles,
				OnLaunch: func(id string, isIsolated bool) {
					modeDesc := "Directorio directo (sin Git)"
					if isIsolated {
						modeDesc = fmt.Sprintf("Worktree aislado (.harness/worktrees/%s)", id)
					}
					icons := tui.GetIcons()
					profileInfo := ""
					if len(profiles) > 0 {
						profileInfo = fmt.Sprintf(" | Perfiles: %s", fmt.Sprint(profiles))
					}
					header := lipgloss.NewStyle().Foreground(tui.ColorSecondary).Bold(true).
						Render(fmt.Sprintf("%s Iniciando chat con %s (Permiso: %s | Workspace: %s%s)...", icons.Rocket, drv.DisplayName(), perm, modeDesc, profileInfo))
					fmt.Fprintln(cmd.OutOrStdout(), header)
				},
			}
			return svc.StartChat(cmd.Context(), req)
		},
	}

	cmd.Flags().StringVarP(&perm, "perm", "m", string(session.PermissionSupervised), "Nivel de permiso: readonly (solo lectura / plan), supervised (con autorización [y/N]), autonomous (autónomo en workspace)")
	cmd.Flags().StringVarP(&provider, "provider", "p", defaultProvider, "Proveedor o agente de terminal (agy, claude, opencode, pi-agent)")
	cmd.Flags().StringSliceVarP(&profiles, "profile", "P", nil, "Perfiles agénticos a activar (ej. -P frontend,data o -P frontend -P data)")
	cmd.Flags().StringVarP(&prompt, "prompt", "i", "", "Objetivo o prompt inicial opcional")
	cmd.Flags().StringVarP(&workDir, "dir", "d", "", "Directorio de trabajo (por defecto el directorio actual)")

	return cmd
}
