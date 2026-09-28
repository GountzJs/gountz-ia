package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"gz-ia/internal/clients/tui"
	"gz-ia/internal/features/memory"
)

var defaultMemoryService memory.Service

func getMemoryService(workDir string) memory.Service {
	if defaultMemoryService != nil {
		return defaultMemoryService
	}
	return memory.NewService(workDir)
}

func newMemoryCmd() *cobra.Command {
	var workDir string

	cmd := &cobra.Command{
		Use:          "memory",
		Short:        "Gestiona memoria semántica local y contexto de decisiones (save, search, list, consolidate)",
		Long:         `Almacena y consulta conocimientos de arquitectura, reglas de negocio y contexto de decisiones con indexación y ranking semántico Okapi BM25.`,
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVarP(&workDir, "dir", "d", "", "Directorio de trabajo o raíz del proyecto")

	// Subcomando save
	var (
		title     string
		content   string
		category  string
		tags      []string
		sessionID string
	)
	saveCmd := &cobra.Command{
		Use:   "save",
		Short: "Guarda una nueva memoria o regla de conocimiento",
		RunE: func(c *cobra.Command, args []string) error {
			targetDir := resolveWorkDir(c.Context(), workDir)
			svc := getMemoryService(targetDir)

			if strings.TrimSpace(title) == "" {
				return fmt.Errorf("el flag --title es obligatorio")
			}
			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("el flag --content es obligatorio")
			}

			rec, err := svc.Save(c.Context(), memory.Record{
				SessionID: sessionID,
				Title:     title,
				Content:   content,
				Category:  category,
				Tags:      tags,
			})
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			scopeMsg := "Global de proyecto (.harness/memory.json)"
			if rec.SessionID != "" {
				scopeMsg = fmt.Sprintf("Sesión '%s'", rec.SessionID)
			}

			msg := lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
				Render(fmt.Sprintf("%s Memoria '%s' guardada exitosamente (ID: %s) [%s]", icons.Check, rec.Title, rec.ID, scopeMsg))
			fmt.Fprintln(c.OutOrStdout(), msg)
			return nil
		},
	}
	saveCmd.Flags().StringVarP(&title, "title", "t", "", "Título descriptivo del conocimiento (obligatorio)")
	saveCmd.Flags().StringVarP(&content, "content", "c", "", "Contenido detallado de la memoria (obligatorio)")
	saveCmd.Flags().StringVar(&category, "category", "", "Categoría (ej. decision, rule, architecture, bugfix)")
	saveCmd.Flags().StringSliceVar(&tags, "tags", nil, "Etiquetas de contexto (separadas por coma)")
	saveCmd.Flags().StringVar(&sessionID, "session", "", "ID de sesión (opcional; si se omite es memoria global)")

	// Subcomando search
	var (
		searchSessionID string
		globalOnly      bool
		limit           int
	)
	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Busca memorias relevantes mediante ranking semántico Okapi BM25",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			targetDir := resolveWorkDir(c.Context(), workDir)
			svc := getMemoryService(targetDir)
			query := args[0]

			results, err := svc.Search(c.Context(), query, memory.SearchOptions{
				SessionID:  searchSessionID,
				GlobalOnly: globalOnly,
				Limit:      limit,
			})
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			if len(results) == 0 {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).
					Render(fmt.Sprintf("%s No se encontraron memorias relevantes para '%s'.", icons.Sparkle, query)))
				return nil
			}

			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(tui.ColorPrimary).
				Render(fmt.Sprintf("%s Resultados de búsqueda para '%s' (%d memorias):", icons.Sparkle, query, len(results))))
			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("──────────────────────────────────────────────────────────────────────────"))

			for _, r := range results {
				scoreStr := fmt.Sprintf("(BM25: %.2f)", r.Score)
				catStr := r.Record.Category
				if catStr == "" {
					catStr = "general"
				}
				tagsStr := ""
				if len(r.Record.Tags) > 0 {
					tagsStr = fmt.Sprintf(" [%s]", strings.Join(r.Record.Tags, ", "))
				}

				fmt.Fprintf(c.OutOrStdout(), "✦ %s  %s%s\n",
					lipgloss.NewStyle().Bold(true).Render(r.Record.Title),
					lipgloss.NewStyle().Foreground(tui.ColorSecondary).Render(scoreStr),
					lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(tagsStr),
				)
				fmt.Fprintf(c.OutOrStdout(), "  Categoría: %s | ID: %s\n", catStr, r.Record.ID)
				fmt.Fprintf(c.OutOrStdout(), "  %s\n\n", strings.ReplaceAll(r.Record.Content, "\n", "\n  "))
			}
			return nil
		},
	}
	searchCmd.Flags().StringVar(&searchSessionID, "session", "", "ID de sesión para incluir su memoria local")
	searchCmd.Flags().BoolVar(&globalOnly, "global-only", false, "Buscar exclusivamente en la memoria del proyecto")
	searchCmd.Flags().IntVarP(&limit, "limit", "l", 10, "Límite de resultados")

	// Subcomando list
	var (
		listSessionID string
		listGlobal    bool
		jsonOutput    bool
	)
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Lista las entradas de memoria guardadas",
		RunE: func(c *cobra.Command, args []string) error {
			targetDir := resolveWorkDir(c.Context(), workDir)
			svc := getMemoryService(targetDir)

			recs, err := svc.List(c.Context(), memory.SearchOptions{
				SessionID:  listSessionID,
				GlobalOnly: listGlobal,
			})
			if err != nil {
				return err
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(recs, "", "  ")
				fmt.Fprintln(c.OutOrStdout(), string(data))
				return nil
			}

			icons := tui.GetIcons()
			if len(recs) == 0 {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).
					Render(fmt.Sprintf("%s No existen registros de memoria guardados.", icons.Sparkle)))
				return nil
			}

			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Bold(true).Foreground(tui.ColorPrimary).
				Render(fmt.Sprintf("%s Catálogo de Memoria (%d entradas):", icons.Lock, len(recs))))
			fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorSubtle).Render("──────────────────────────────────────────────────────────────────────────"))

			for _, r := range recs {
				catStr := r.Category
				if catStr == "" {
					catStr = "general"
				}
				tagsStr := ""
				if len(r.Tags) > 0 {
					tagsStr = fmt.Sprintf(" [%s]", strings.Join(r.Tags, ", "))
				}

				fmt.Fprintf(c.OutOrStdout(), "✦ %s%s\n",
					lipgloss.NewStyle().Bold(true).Render(r.Title),
					lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(tagsStr),
				)
				fmt.Fprintf(c.OutOrStdout(), "  ID: %s | Categoría: %s\n", r.ID, catStr)
				fmt.Fprintf(c.OutOrStdout(), "  %s\n\n", strings.ReplaceAll(r.Content, "\n", "\n  "))
			}
			return nil
		},
	}
	listCmd.Flags().StringVar(&listSessionID, "session", "", "ID de sesión")
	listCmd.Flags().BoolVar(&listGlobal, "global-only", false, "Listar exclusivamente memoria del proyecto")
	listCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Emite el catálogo como JSON")

	// Subcomando consolidate
	consolidateCmd := &cobra.Command{
		Use:   "consolidate <session_id>",
		Short: "Promueve las memorias de una sesión hacia la memoria global del proyecto",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			targetDir := resolveWorkDir(c.Context(), workDir)
			svc := getMemoryService(targetDir)
			sessID := args[0]

			recs, err := svc.Consolidate(c.Context(), sessID)
			if err != nil {
				return err
			}

			icons := tui.GetIcons()
			if len(recs) == 0 {
				fmt.Fprintln(c.OutOrStdout(), lipgloss.NewStyle().Foreground(tui.ColorMuted).
					Render(fmt.Sprintf("%s La sesión '%s' no tiene memorias locales para consolidar.", icons.Sparkle, sessID)))
				return nil
			}

			msg := lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).
				Render(fmt.Sprintf("%s Se consolidaron %d memorias de la sesión '%s' a la memoria global del proyecto.", icons.Check, len(recs), sessID))
			fmt.Fprintln(c.OutOrStdout(), msg)
			return nil
		},
	}

	cmd.AddCommand(saveCmd)
	cmd.AddCommand(searchCmd)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(consolidateCmd)

	return cmd
}
