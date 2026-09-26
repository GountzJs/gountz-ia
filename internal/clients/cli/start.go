package cli

import (
	"gz-ia/internal/clients/tui"

	"github.com/spf13/cobra"
)

var runTUI = func() error {
	client := tui.New(nil, nil).WithUpdater(getUpdaterService())
	return client.Run()
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Inicia la interfaz interactiva en terminal (TUI)",
		Long:  `Lanza el banner visual interactivo con estética Fastfetch y el menú interactivo inicial.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI()
		},
	}
}
