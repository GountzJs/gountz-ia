package cli

import (
	"fmt"
	"gz-ia/internal/clients/tui"
	"gz-ia/internal/version"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Muestra la versión de gz-ia",
		Run: func(cmd *cobra.Command, args []string) {
			icons := tui.GetIcons()
			title := lipgloss.NewStyle().Foreground(tui.ColorPrimary).Bold(true).Render("gz-ia")
			ver := lipgloss.NewStyle().Foreground(tui.ColorWhite).Render("v" + version.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s (commit: %s, date: %s)\n", icons.Sparkle, title, ver, version.Commit, version.Date)
		},
	}
}
