package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"gz-ia/internal/clients/tui"
	"gz-ia/internal/features/logger"
	"gz-ia/internal/features/metrics"
	"gz-ia/internal/features/session"
	"gz-ia/internal/features/workspace"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var defaultSessionStore session.Store
var defaultSessionKiller session.ProcessKiller
var defaultSessionWorkspace workspace.Provider
var defaultSessionMetrics metrics.Service
var defaultSessionLogger logger.Service

func resolveWorkDir(ctx context.Context, dir string) string {
	if defaultSessionWorkspace != nil {
		return defaultSessionWorkspace.ResolveProjectRoot(ctx, dir)
	}
	return workspace.ResolveProjectRoot(ctx, dir)
}

func getSessionService(workDir string) session.Service {
	var opts []session.Option
	if defaultSessionRunner != nil {
		opts = append(opts, session.WithRunner(defaultSessionRunner))
	}
	if defaultSessionStore != nil {
		opts = append(opts, session.WithStore(defaultSessionStore))
	}
	if defaultSessionKiller != nil {
		opts = append(opts, session.WithKiller(defaultSessionKiller))
	}
	if defaultSessionWorkspace != nil {
		opts = append(opts, session.WithWorkspace(defaultSessionWorkspace))
	}
	if defaultSessionMetrics != nil {
		opts = append(opts, session.WithMetrics(defaultSessionMetrics))
	}
	if defaultSessionLogger != nil {
		opts = append(opts, session.WithLogger(defaultSessionLogger))
	}
	return session.NewService(workDir, opts...)
}

func newSessionCmd() *cobra.Command {
	var workDir string

	cmd := &cobra.Command{
		Use:          "session",
		Short:        "Gestiona sesiones agénticas locales (list, kill, resume, delete, show, diff, merge, read, get, path)",
		Long:         `Permite listar, inspeccionar, reanudar, eliminar, comparar, fusionar u obtener la ruta de sesiones registradas en el proyecto actual.`,
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVarP(&workDir, "dir", "d", "", "Directorio de trabajo (por defecto el directorio actual)")

	// Subcomando list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Lista las sesiones agénticas del proyecto",
		RunE: func(c *cobra.Command, args []string) error {
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)
			sessions, err := svc.List(c.Context())
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			if len(sessions) == 0 {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(fmt.Sprintf("%s No hay sesiones registradas en este proyecto.", icons.Sparkle)))
				return nil
			}

			header := fmt.Sprintf("%-10s  %-12s  %-8s  %-12s  %-10s  %-20s  %-10s", "ID", "ESTADO", "PID", "PERMISO", "MODO", "INICIO", "DURACIÓN")
			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSecondary).Render(header))
			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("──────────────────────────────────────────────────────────────────────────────────────────"))

			for _, s := range sessions {
				var statusStyle lipgloss.Style
				switch s.Status {
				case session.StatusRunning:
					statusStyle = lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true)
				case session.StatusKilled:
					statusStyle = lipgloss.NewStyle().Foreground(tui.ColorWarning)
				case session.StatusFailed:
					statusStyle = lipgloss.NewStyle().Foreground(tui.ColorDanger)
				default:
					statusStyle = lipgloss.NewStyle().Foreground(tui.ColorWhite)
				}

				durationStr := "-"
				if s.DurationMs > 0 {
					d := time.Duration(s.DurationMs) * time.Millisecond
					durationStr = d.Round(time.Second).String()
				} else if s.Status == session.StatusRunning {
					d := time.Since(s.StartedAt)
					durationStr = d.Round(time.Second).String()
				}

				pidStr := "-"
				if s.PID > 0 {
					pidStr = fmt.Sprintf("%d", s.PID)
				}

				modeStr := "Directo"
				if s.IsIsolated {
					modeStr = "Worktree"
				}

				startedStr := s.StartedAt.Format("2006-01-02 15:04:05")

				line := fmt.Sprintf("%-10s  %-12s  %-8s  %-12s  %-10s  %-20s  %-10s",
					s.ID,
					statusStyle.Render(string(s.Status)),
					pidStr,
					s.PermissionLevel,
					modeStr,
					startedStr,
					durationStr,
				)
				fmt.Fprintln(c.OutOrStdout(), line)
			}
			return nil
		},
	}

	// Subcomando kill
	killCmd := &cobra.Command{
		Use:   "kill <id>",
		Short: "Finaliza una sesión agéntica activa por su ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)
			rec, err := svc.GetRecord(c.Context(), id)
			if err != nil {
				return err
			}
			if err := svc.Kill(c.Context(), id); err != nil {
				return err
			}

			icons := tui.GetIcons()
			msg := lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
				Render(fmt.Sprintf("%s Sesión '%s' (PID %d) finalizada con éxito.", icons.Check, rec.ID, rec.PID))
			fmt.Fprintln(c.OutOrStdout(), msg)
			return nil
		},
	}

	// Subcomando resume (revivir)
	resumeCmd := &cobra.Command{
		Use:     "resume <id>",
		Aliases: []string{"continue"},
		Short:   "Revive / reanuda una sesión existente con agy",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)
			rec, err := svc.GetRecord(c.Context(), id)
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			header := lipgloss.NewStyle().Foreground(tui.ColorSecondary).Bold(true).
				Render(fmt.Sprintf("%s Reanudando sesión [%s] en [%s]...", icons.Sync, rec.ID, rec.WorkingDir))
			fmt.Fprintln(c.OutOrStdout(), header)

			return svc.Resume(c.Context(), id)
		},
	}

	// Subcomando delete (rm)
	deleteCmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"rm"},
		Short:   "Elimina una sesión del registro local",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)
			if err := svc.Delete(c.Context(), id); err != nil {
				return err
			}

			icons := tui.GetIcons()
			msg := lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
				Render(fmt.Sprintf("%s Sesión '%s' eliminada del registro.", icons.Check, id))
			fmt.Fprintln(c.OutOrStdout(), msg)
			return nil
		},
	}

	// Subcomando show
	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Muestra la información detallada de una sesión",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)
			rec, err := svc.GetRecord(c.Context(), id)
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			header := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorPrimary).Render(fmt.Sprintf("%s Detalle de Sesión: %s", icons.Terminal, rec.ID))
			fmt.Fprintln(c.OutOrStdout(), header)
			fmt.Fprintf(c.OutOrStdout(), "  Estado:     %s\n", rec.Status)
			fmt.Fprintf(c.OutOrStdout(), "  PID:        %d\n", rec.PID)
			if rec.Provider != "" {
				fmt.Fprintf(c.OutOrStdout(), "  Proveedor:  %s\n", rec.Provider)
			}
			fmt.Fprintf(c.OutOrStdout(), "  Permisos:   %s\n", rec.PermissionLevel)
			fmt.Fprintf(c.OutOrStdout(), "  Directorio: %s\n", rec.WorkingDir)
			if rec.IsIsolated {
				fmt.Fprintf(c.OutOrStdout(), "  Modo:       Worktree aislado (%s)\n", rec.WorktreeDir)
				if rec.BranchName != "" {
					fmt.Fprintf(c.OutOrStdout(), "  Rama:       %s\n", rec.BranchName)
				}
			} else {
				fmt.Fprintf(c.OutOrStdout(), "  Modo:       Directo (sin Git)\n")
			}
			if rec.InitialPrompt != "" {
				fmt.Fprintf(c.OutOrStdout(), "  Prompt:     %s\n", rec.InitialPrompt)
			}
			fmt.Fprintf(c.OutOrStdout(), "  Inicio:     %s\n", rec.StartedAt.Format(time.RFC3339))
			if rec.FinishedAt != nil {
				fmt.Fprintf(c.OutOrStdout(), "  Fin:        %s\n", rec.FinishedAt.Format(time.RFC3339))
			}
			fmt.Fprintf(c.OutOrStdout(), "  Exit Code:  %d\n", rec.ExitCode)
			return nil
		},
	}

	// Subcomando path
	pathCmd := &cobra.Command{
		Use:   "path <id>",
		Short: "Imprime la ruta absoluta del espacio de trabajo de la sesión",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)
			p, err := svc.Path(c.Context(), id)
			if err != nil {
				return err
			}
			fmt.Fprintln(c.OutOrStdout(), p)
			return nil
		},
	}

	// Subcomando diff
	var statFlag bool
	diffCmd := &cobra.Command{
		Use:   "diff <id>",
		Short: "Muestra las diferencias de código producidas en la sesión",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)
			out, err := svc.Diff(c.Context(), id, statFlag)
			if err != nil {
				return err
			}
			if strings.TrimSpace(out) == "" {
				icons := tui.GetIcons()
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(fmt.Sprintf("%s No hay cambios registrados en la sesión.", icons.Sparkle)))
				return nil
			}
			fmt.Fprintln(c.OutOrStdout(), out)
			return nil
		},
	}
	diffCmd.Flags().BoolVar(&statFlag, "stat", false, "Muestra solo el resumen estadístico de archivos modificados")

	// Subcomando merge
	var squashFlag bool
	var noCommitFlag bool
	mergeCmd := &cobra.Command{
		Use:   "merge <id>",
		Short: "Integra los cambios de una sesión aislada a la rama de trabajo principal",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)
			res, err := svc.Merge(c.Context(), id, session.MergeOptions{
				Squash:   squashFlag,
				NoCommit: noCommitFlag,
			})
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			if res.AlreadyUpToDate {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(fmt.Sprintf("%s La rama ya está actualizada. No hay cambios pendientes para fusionar.", icons.Sparkle)))
				return nil
			}

			successMsg := lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
				Render(fmt.Sprintf("%s Fusión completada con éxito para la sesión '%s'.", icons.Check, id))
			fmt.Fprintln(c.OutOrStdout(), successMsg)

			if len(res.FilesIntegrated) > 0 {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSecondary).Render("Archivos integrados:"))
				for _, f := range res.FilesIntegrated {
					fmt.Fprintf(c.OutOrStdout(), "  • %s\n", f)
				}
			}
			if squashFlag && noCommitFlag {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorWarning).Render("Nota: Los cambios quedaron preparados en el stage sin comitear (--squash --no-commit)."))
			} else if noCommitFlag {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorWarning).Render("Nota: Los cambios quedaron preparados en el stage sin comitear (--no-commit)."))
			}
			return nil
		},
	}
	mergeCmd.Flags().BoolVar(&squashFlag, "squash", false, "Condensa los cambios en un único commit sin merge commit")
	mergeCmd.Flags().BoolVar(&noCommitFlag, "no-commit", false, "Deja los cambios preparados en el stage del repo principal sin comitear")

	// Subcomando read
	var readStatFlag bool
	readCmd := &cobra.Command{
		Use:   "read <id>",
		Short: "Inspecciona el contenido y diff del worktree de la sesión",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)
			out, err := svc.Read(c.Context(), id, readStatFlag)
			if err != nil {
				return err
			}
			if strings.TrimSpace(out) == "" {
				icons := tui.GetIcons()
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(fmt.Sprintf("%s No hay cambios registrados en la sesión.", icons.Sparkle)))
				return nil
			}
			fmt.Fprintln(c.OutOrStdout(), out)
			return nil
		},
	}
	readCmd.Flags().BoolVar(&readStatFlag, "stat", false, "Muestra solo el resumen estadístico de archivos modificados")

	// Subcomando get
	var getSquashFlag bool
	var getNoCommitFlag bool
	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Trae e integra los cambios del worktree de sesión hacia el directorio de trabajo activo",
		Long: `Trae e integra los cambios producidos en el worktree de sesión hacia la rama activa del repositorio.

Semántica de integración:
1. Si hay cambios pendientes en el worktree, crea un commit de seguridad en la rama 'harness/<id>'.
2. Ejecuta un git merge (o git merge --squash con --squash, y sin commit si se pasa --no-commit) de la rama 'harness/<id>' en la rama base activa del repositorio.
3. Si la rama base avanzó y existen conflictos, Git se detiene sin sobreescribir tus archivos, informa los archivos en conflicto y mantiene el worktree de la sesión intacto para resolución manual ('gz-ia session path <id>') o abortar ('git merge --abort').`,
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)
			res, err := svc.Get(c.Context(), id, session.MergeOptions{
				Squash:   getSquashFlag,
				NoCommit: getNoCommitFlag,
			})
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			if res.AlreadyUpToDate {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(fmt.Sprintf("%s La rama ya está actualizada. No hay cambios pendientes para fusionar.", icons.Sparkle)))
				return nil
			}

			successMsg := lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
				Render(fmt.Sprintf("%s Cambios del worktree traídos e integrados con éxito para la sesión '%s'.", icons.Check, id))
			fmt.Fprintln(c.OutOrStdout(), successMsg)

			if len(res.FilesIntegrated) > 0 {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSecondary).Render("Archivos integrados:"))
				for _, f := range res.FilesIntegrated {
					fmt.Fprintf(c.OutOrStdout(), "  • %s\n", f)
				}
			}
			if getSquashFlag && getNoCommitFlag {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorWarning).Render("Nota: Los cambios quedaron preparados en el stage sin comitear (--squash --no-commit)."))
			} else if getNoCommitFlag {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorWarning).Render("Nota: Los cambios quedaron preparados en el stage sin comitear (--no-commit)."))
			}
			return nil
		},
	}
	getCmd.Flags().BoolVar(&getSquashFlag, "squash", false, "Condensa los cambios en un único commit sin merge commit")
	getCmd.Flags().BoolVar(&getNoCommitFlag, "no-commit", false, "Deja los cambios preparados en el stage del repo principal sin comitear")

	// Subcomando metrics
	var jsonFlag bool
	var detailedFlag bool
	metricsCmd := &cobra.Command{
		Use:   "metrics <id>",
		Short: "Muestra métricas y telemetría de tokens, herramientas y subagentes de una sesión",
		Long:  `Analiza las trazas de Antigravity (transcript.jsonl y history.jsonl) para calcular la duración exacta, pasos ejecutados, desglose de tokens y uso de herramientas tanto de la sesión principal como de todos los subagentes invocados recursivamente.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)
			m, err := svc.Metrics(c.Context(), id)
			if err != nil {
				return err
			}

			if jsonFlag {
				data, err := json.MarshalIndent(m, "", "  ")
				if err != nil {
					return fmt.Errorf("error al serializar métricas a JSON: %w", err)
				}
				fmt.Fprintln(c.OutOrStdout(), string(data))
				return nil
			}

			icons := tui.GetIcons()
			titleStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorPrimary)
			sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSecondary)
			keyStyle := lipgloss.NewStyle().Foreground(tui.ColorSecondary)
			valStyle := lipgloss.NewStyle().Foreground(tui.ColorWhite)
			mutedStyle := lipgloss.NewStyle().Foreground(tui.ColorMuted)
			divider := lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("──────────────────────────────────────────────────────────────────────────")

			fmt.Fprintln(c.OutOrStdout(), titleStyle.Render(fmt.Sprintf("%s Telemetría y Métricas de Sesión: %s", icons.Sparkle, m.SessionID)))
			fmt.Fprintln(c.OutOrStdout(), divider)

			durStr := m.TotalDuration.Round(time.Millisecond).String()
			if m.TotalDuration >= time.Second {
				durStr = m.TotalDuration.Round(time.Second).String()
			}

			fmt.Fprintf(c.OutOrStdout(), "  %s %s\n", keyStyle.Render("Conversación:"), valStyle.Render(m.ConversationID))
			fmt.Fprintf(c.OutOrStdout(), "  %s %s\n", keyStyle.Render("Duración Total:"), valStyle.Render(durStr))
			fmt.Fprintf(c.OutOrStdout(), "  %s %s\n", keyStyle.Render("Pasos de Traza:"), valStyle.Render(fmt.Sprintf("%d", m.StepsCount)))
			if !m.StartedAt.IsZero() {
				fmt.Fprintf(c.OutOrStdout(), "  %s %s\n", keyStyle.Render("Inicio:        "), valStyle.Render(m.StartedAt.Format("2006-01-02 15:04:05")))
			}
			if m.FinishedAt != nil {
				fmt.Fprintf(c.OutOrStdout(), "  %s %s\n", keyStyle.Render("Fin:           "), valStyle.Render(m.FinishedAt.Format("2006-01-02 15:04:05")))
			}

			// Desglose de tokens
			fmt.Fprintln(c.OutOrStdout())
			fmt.Fprintln(c.OutOrStdout(), sectionStyle.Render(fmt.Sprintf("%s Consumo Estimado de Tokens (~4 caracteres/token):", icons.Bolt)))
			fmt.Fprintf(c.OutOrStdout(), "  • %-18s %s\n", "Prompt:", valStyle.Render(fmt.Sprintf("%d tokens", m.Tokens.PromptTokens)))
			fmt.Fprintf(c.OutOrStdout(), "  • %-18s %s\n", "Thinking:", valStyle.Render(fmt.Sprintf("%d tokens", m.Tokens.ThinkingTokens)))
			fmt.Fprintf(c.OutOrStdout(), "  • %-18s %s\n", "Output / Tools:", valStyle.Render(fmt.Sprintf("%d tokens", m.Tokens.CompletionTokens)))
			totalTokStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSuccess)
			fmt.Fprintf(c.OutOrStdout(), "  • %-18s %s\n", "Total Sesión:", totalTokStyle.Render(fmt.Sprintf("%d tokens", m.Tokens.TotalTokens)))

			// Resumen de herramientas
			fmt.Fprintln(c.OutOrStdout())
			fmt.Fprintln(c.OutOrStdout(), sectionStyle.Render(fmt.Sprintf("%s Resumen de Herramientas Usadas:", icons.Terminal)))
			if len(m.ToolCalls) == 0 {
				fmt.Fprintln(c.OutOrStdout(), mutedStyle.Render("  (Ninguna llamada a herramientas registrada en la sesión principal)"))
			} else {
				var toolNames []string
				for name := range m.ToolCalls {
					toolNames = append(toolNames, name)
				}
				sort.Strings(toolNames)
				for _, name := range toolNames {
					fmt.Fprintf(c.OutOrStdout(), "  • %-26s %s llamadas\n", name+":", valStyle.Render(fmt.Sprintf("%d", m.ToolCalls[name])))
				}
			}

			// Desglose de Subagentes
			fmt.Fprintln(c.OutOrStdout())
			if len(m.Subagents) == 0 {
				fmt.Fprintln(c.OutOrStdout(), sectionStyle.Render(fmt.Sprintf("%s Subagentes Invocados: 0", icons.Robot)))
				fmt.Fprintln(c.OutOrStdout(), mutedStyle.Render("  No se invocaron subagentes en esta sesión."))
			} else {
				fmt.Fprintln(c.OutOrStdout(), sectionStyle.Render(fmt.Sprintf("%s Subagentes Invocados (%d):", icons.Robot, len(m.Subagents))))
				for i, sub := range m.Subagents {
					subDurStr := sub.Duration.Round(time.Millisecond).String()
					if sub.Duration >= time.Second {
						subDurStr = sub.Duration.Round(time.Second).String()
					}

					var statusBadge lipgloss.Style
					switch sub.Status {
					case "completed":
						statusBadge = lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true)
					case "failed":
						statusBadge = lipgloss.NewStyle().Foreground(tui.ColorDanger).Bold(true)
					default:
						statusBadge = lipgloss.NewStyle().Foreground(tui.ColorWarning).Bold(true)
					}

					fmt.Fprintf(c.OutOrStdout(), "\n  [%d] %s (%s) — Estado: %s\n",
						i+1,
						lipgloss.NewStyle().Bold(true).Foreground(tui.ColorWhite).Render(sub.Role),
						mutedStyle.Render(sub.TypeName),
						statusBadge.Render(sub.Status),
					)
					fmt.Fprintf(c.OutOrStdout(), "      ConversationID: %s\n", sub.ConversationID)
					fmt.Fprintf(c.OutOrStdout(), "      Duración: %s | Pasos: %d\n", subDurStr, sub.StepsCount)
					fmt.Fprintf(c.OutOrStdout(), "      Tokens: Prompt: %d | Thinking: %d | Output: %d | Total: %d\n",
						sub.Tokens.PromptTokens, sub.Tokens.ThinkingTokens, sub.Tokens.CompletionTokens, sub.Tokens.TotalTokens)

					if len(sub.ToolCalls) > 0 {
						var subToolNames []string
						for tName := range sub.ToolCalls {
							subToolNames = append(subToolNames, fmt.Sprintf("%s (%d)", tName, sub.ToolCalls[tName]))
						}
						sort.Strings(subToolNames)
						fmt.Fprintf(c.OutOrStdout(), "      Herramientas: %s\n", strings.Join(subToolNames, ", "))
					}

					if detailedFlag {
						if sub.Prompt != "" {
							fmt.Fprintf(c.OutOrStdout(), "      Prompt: %s\n", sub.Prompt)
						}
						if !sub.StartedAt.IsZero() {
							fmt.Fprintf(c.OutOrStdout(), "      Inicio: %s\n", sub.StartedAt.Format("2006-01-02 15:04:05"))
						}
						if sub.FinishedAt != nil {
							fmt.Fprintf(c.OutOrStdout(), "      Fin:    %s\n", sub.FinishedAt.Format("2006-01-02 15:04:05"))
						}
					}
				}
			}

			return nil
		},
	}
	metricsCmd.Flags().BoolVar(&jsonFlag, "json", false, "Emite el objeto JSON puro a stdout para consumo programático")
	metricsCmd.Flags().BoolVar(&detailedFlag, "detailed", false, "Muestra información extendida con prompts y timestamps de subagentes")

	// Subcomando log
	var logAction string
	var logStage string
	var logStatus string
	var logAgent string
	var logRole string
	var logError string
	var logDuration int64

	logCmd := &cobra.Command{
		Use:   "log <id>",
		Short: "Registra un evento de observabilidad y estado para una sesión",
		Long:  `Permite a agentes, subagentes o procesos externos registrar etapas (READ, PENDING, FINISH) y estados de acciones en el log de eventos de la sesión.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)

			stageUpper := logger.Stage(strings.ToUpper(strings.TrimSpace(logStage)))
			if !stageUpper.IsValid() {
				return fmt.Errorf("etapa inválida: '%s' (válidos: READ, PENDING, FINISH)", logStage)
			}

			var statusPtr *logger.Status
			if c.Flags().Changed("status") && strings.TrimSpace(logStatus) != "" {
				sUpper := logger.Status(strings.ToUpper(strings.TrimSpace(logStatus)))
				if !sUpper.IsValid() {
					return fmt.Errorf("estado inválido: '%s' (válidos: OK, FAILED)", logStatus)
				}
				statusPtr = &sUpper
			}

			var durationPtr *int64
			if c.Flags().Changed("duration") {
				durationPtr = &logDuration
			}

			evt := &logger.Event{
				SessionID:  id,
				AgentID:    logAgent,
				Role:       logRole,
				Action:     logAction,
				Stage:      stageUpper,
				Status:     statusPtr,
				Error:      logError,
				DurationMs: durationPtr,
			}

			if err := svc.LogEvent(c.Context(), evt); err != nil {
				return err
			}

			icons := tui.GetIcons()
			msg := lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
				Render(fmt.Sprintf("%s Evento registrado exitosamente para la sesión '%s'.", icons.Check, id))
			fmt.Fprintln(c.OutOrStdout(), msg)
			return nil
		},
	}

	logCmd.Flags().StringVarP(&logAction, "action", "a", "", "Descripción de la acción (requerido)")
	logCmd.Flags().StringVarP(&logStage, "stage", "s", "", "Etapa (READ, PENDING, FINISH) (requerido)")
	logCmd.Flags().StringVar(&logStatus, "status", "", "Estado final (OK, FAILED) (opcional, por defecto null)")
	logCmd.Flags().StringVar(&logAgent, "agent", "orchestrator", "Identificador del agente/subagente")
	logCmd.Flags().StringVarP(&logRole, "role", "r", "", "Rol del agente")
	logCmd.Flags().StringVar(&logError, "error", "", "Detalle del error si falló")
	logCmd.Flags().Int64Var(&logDuration, "duration", 0, "Duración en milisegundos")

	_ = logCmd.MarkFlagRequired("action")
	_ = logCmd.MarkFlagRequired("stage")

	// Subcomando logs
	var followFlag bool
	var logsJsonFlag bool

	logsCmd := &cobra.Command{
		Use:   "logs <id>",
		Short: "Muestra o transmite el registro de eventos de una sesión",
		Long:  `Permite consultar cronológicamente los eventos de observabilidad de una sesión o transmitirlos en tiempo real (--follow).`,
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			workDir = resolveWorkDir(c.Context(), workDir)
			svc := getSessionService(workDir)

			formatEventLine := func(evt logger.Event) string {
				timeStr := evt.Timestamp.Local().Format("15:04:05")
				timeBadge := lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render(fmt.Sprintf("[%s]", timeStr))

				roleAgent := evt.AgentID
				if evt.Role != "" && evt.AgentID != "" && evt.Role != evt.AgentID {
					roleAgent = fmt.Sprintf("%s/%s", evt.Role, evt.AgentID)
				} else if evt.Role != "" {
					roleAgent = evt.Role
				}
				if roleAgent == "" {
					roleAgent = "orchestrator"
				}
				roleBadge := lipgloss.NewStyle().Foreground(tui.ColorSecondary).Bold(true).Render(fmt.Sprintf("[%s]", roleAgent))

				var stageBadge string
				switch evt.Stage {
				case logger.StageRead:
					stageBadge = lipgloss.NewStyle().Foreground(tui.ColorPrimary).Bold(true).Render("[READ]")
				case logger.StagePending:
					stageBadge = lipgloss.NewStyle().Foreground(tui.ColorWarning).Bold(true).Render("[PENDING]")
				case logger.StageFinish:
					stageBadge = lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).Render("[FINISH]")
				default:
					stageBadge = fmt.Sprintf("[%s]", evt.Stage)
				}

				actionStr := lipgloss.NewStyle().Foreground(tui.ColorWhite).Render(evt.Action)

				var statusStr string
				if evt.Status == nil {
					statusStr = lipgloss.NewStyle().Foreground(tui.ColorMuted).Render("(status: null)")
				} else if *evt.Status == logger.StatusOK {
					statusStr = lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).Render("(status: OK)")
				} else if *evt.Status == logger.StatusFailed {
					statusStr = lipgloss.NewStyle().Foreground(tui.ColorDanger).Bold(true).Render("(status: FAILED)")
				}

				return fmt.Sprintf("%s %s %s %s ... %s", timeBadge, roleBadge, stageBadge, actionStr, statusStr)
			}

			renderEvent := func(evt logger.Event) error {
				if logsJsonFlag {
					data, err := json.Marshal(evt)
					if err != nil {
						return err
					}
					fmt.Fprintln(c.OutOrStdout(), string(data))
					return nil
				}
				fmt.Fprintln(c.OutOrStdout(), formatEventLine(evt))
				return nil
			}

			if followFlag {
				ch, err := svc.WatchEvents(c.Context(), id)
				if err != nil {
					return err
				}
				for evt := range ch {
					if err := renderEvent(evt); err != nil {
						return err
					}
				}
				return nil
			}

			events, err := svc.GetEvents(c.Context(), id)
			if err != nil {
				return err
			}

			if len(events) == 0 && !logsJsonFlag {
				icons := tui.GetIcons()
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(fmt.Sprintf("%s No hay eventos registrados para la sesión '%s'.", icons.Sparkle, id)))
				return nil
			}

			for _, evt := range events {
				if err := renderEvent(evt); err != nil {
					return err
				}
			}

			return nil
		},
	}

	logsCmd.Flags().BoolVarP(&followFlag, "follow", "f", false, "Transmisión en tiempo real (Watch)")
	logsCmd.Flags().BoolVar(&logsJsonFlag, "json", false, "Formato JSON lines a stdout")

	pruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Reconcilia y elimina sesiones, ramas de Git y worktrees huérfanos",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			targetWorkDir := resolveWorkDir(ctx, workDir)
			svc := getSessionService(targetWorkDir)

			res, err := svc.Prune(ctx)
			if err != nil {
				return fmt.Errorf("error al reconciliar elementos huérfanos: %w", err)
			}

			if len(res.DeletedBranches) == 0 && len(res.PrunedWorktrees) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), tui.StyleSuccess.Render("✓ No se encontraron ramas de Git ni worktrees huérfanos. Todo está sincronizado."))
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), tui.StyleTitle.Render("✧ Reconciliación de Basura Huérfana:"))
			for _, b := range res.DeletedBranches {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s Rama huérfana eliminada: %s\n", tui.StyleSuccess.Render("✓"), b)
			}
			for _, w := range res.PrunedWorktrees {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s Worktree huérfano podado: %s\n", tui.StyleSuccess.Render("✓"), w)
			}
			return nil
		},
	}
	pruneCmd.Flags().StringVarP(&workDir, "dir", "d", "", "Directorio de trabajo")

	cmd.AddCommand(listCmd)
	cmd.AddCommand(killCmd)
	cmd.AddCommand(resumeCmd)
	cmd.AddCommand(deleteCmd)
	cmd.AddCommand(showCmd)
	cmd.AddCommand(diffCmd)
	cmd.AddCommand(mergeCmd)
	cmd.AddCommand(readCmd)
	cmd.AddCommand(getCmd)
	cmd.AddCommand(pathCmd)
	cmd.AddCommand(metricsCmd)
	cmd.AddCommand(logCmd)
	cmd.AddCommand(logsCmd)
	cmd.AddCommand(pruneCmd)

	return cmd
}
