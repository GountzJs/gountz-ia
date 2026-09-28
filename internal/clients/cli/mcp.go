package cli

import (
	"fmt"
	"os"

	"gz-ia/internal/features/tooling"
	"gz-ia/internal/version"
	"gz-ia/packages/orchy"
	"gz-ia/packages/orchy/batteries/observability"
	"gz-ia/packages/orchy/batteries/worktree"

	"github.com/spf13/cobra"
)

func newMcpCmd() *cobra.Command {
	var (
		sessionID  string
		toolingDir string
		workDir    string
		allowGet   bool
		toolkits   []string
		profile    string
	)

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Inicia el servidor MCP nativo de gz-ia sobre Stdio (JSON-RPC 2.0)",
		Long: `Ejecuta el microkernel Orchy en modo servidor MCP por Stdio.
Expone herramientas del microkernel (worktree_read y herramientas de toolkits) a agentes de IA como Antigravity o Claude Code.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()
			resolvedWorkDir := resolveWorkDir(ctx, workDir)
			targetDir := resolvedWorkDir

			// 1. Inicializar microkernel
			kernel := orchy.NewKernel()

			// 2. Si se especificó sessionID, localizar metadatos de sesión
			var activeProfiles []string
			if profile != "" {
				activeProfiles = append(activeProfiles, profile)
			}

			if sessionID != "" {
				sessSvc := getSessionService(resolvedWorkDir)
				if rec, err := sessSvc.GetRecord(ctx, sessionID); err == nil && rec != nil {
					if rec.IsIsolated && rec.WorktreeDir != "" {
						targetDir = rec.WorktreeDir
					}
					if len(rec.Profiles) > 0 {
						activeProfiles = append(activeProfiles, rec.Profiles...)
					}
				}
			}

			// 3. Registrar baterías nativas (worktree y observability)
			worktreePlugin := worktree.NewWorktreePlugin(targetDir, worktree.WithAllowAgentGet(allowGet))
			if err := kernel.Use(worktreePlugin); err != nil {
				fmt.Fprintf(os.Stderr, "[gz-ia mcp] Advertencia al registrar batería worktree: %v\n", err)
			}

			obsPlugin := observability.NewObservabilityPlugin(resolvedWorkDir, observability.WithSessionID(sessionID))
			if err := kernel.Use(obsPlugin); err != nil {
				fmt.Fprintf(os.Stderr, "[gz-ia mcp] Advertencia al registrar batería observability: %v\n", err)
			}

			// 4. Registrar herramientas de Tooling si hay toolkits o perfiles
			toolingSvc := tooling.NewService(toolingDir)
			allToolkitIDs := append([]string{}, toolkits...)

			for _, pName := range activeProfiles {
				if pCfg, err := toolingSvc.GetProfile(ctx, pName); err == nil && pCfg != nil {
					allToolkitIDs = append(allToolkitIDs, pCfg.Toolkits...)
				}
			}

			if len(allToolkitIDs) > 0 {
				if composed, err := toolingSvc.ComposeToolkits(ctx, allToolkitIDs); err == nil && composed != nil {
					if err := toolingSvc.RegisterToolsInKernel(ctx, kernel, composed, targetDir); err != nil {
						fmt.Fprintf(os.Stderr, "[gz-ia mcp] Advertencia al registrar herramientas de tooling: %v\n", err)
					}
				}
			}

			// 5. Iniciar microkernel
			if err := kernel.Boot(ctx); err != nil {
				return fmt.Errorf("error al iniciar microkernel Orchy: %w", err)
			}
			defer func() {
				_ = kernel.Shutdown(ctx)
			}()

			// 6. Servir MCP por Stdio
			server := orchy.NewMcpServer(kernel, c.InOrStdin(), c.OutOrStdout())
			server.SetServerInfo("gz-ia", version.Version)

			return server.Serve(ctx)
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Identificador de la sesión activa de gz-ia")
	cmd.Flags().StringVar(&toolingDir, "tooling", "", "Directorio de tooling (por defecto ~/.config/gz-ia/tooling)")
	cmd.Flags().StringVarP(&workDir, "dir", "d", "", "Directorio de trabajo base")
	cmd.Flags().BoolVar(&allowGet, "allow-get", false, "Habilita la herramienta worktree_get para el agente")
	cmd.Flags().StringSliceVar(&toolkits, "toolkit", nil, "Toolkits a activar explícitamente en el microkernel")
	cmd.Flags().StringVarP(&profile, "profile", "P", "", "Perfil de tooling a activar")

	return cmd
}
