package cli

import (
	"fmt"
	"gz-ia/internal/version"
	"os"

	"github.com/spf13/cobra"
)

// NewRootCmd crea e inicializa el árbol de comandos de la CLI.
func NewRootCmd() *cobra.Command {
	startCmd := newStartCmd()
	versionCmd := newVersionCmd()

	cmd := &cobra.Command{
		Use:   "gz-ia",
		Short: "gz-ia — Base agéntica desacoplada",
		Long: `gz-ia es la infraestructura base para orquestación y desarrollo
de flujos agénticos modulares en Go.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(c *cobra.Command, args []string) error {
			// Por defecto al invocar sin argumentos, se inicia la TUI interactiva
			return startCmd.RunE(c, args)
		},
	}

	cmd.Version = version.Version
	cmd.AddCommand(startCmd)
	cmd.AddCommand(versionCmd)
	cmd.AddCommand(newChatCmd())
	cmd.AddCommand(newSessionCmd())
	cmd.AddCommand(newProfileCmd())
	cmd.AddCommand(newVaultCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newMcpCmd())

	return cmd
}

// Execute ejecuta el comando raíz de la CLI.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
