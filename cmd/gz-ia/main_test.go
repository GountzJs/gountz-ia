package main

import (
	"os"
	"testing"

	"gz-ia/internal/clients/cli"
)

func TestMainExecution(t *testing.T) {
	// Guardamos os.Args original y lo restauramos al final
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Ejecutar comando con flag de ayuda
	os.Args = []string{"gz-ia", "version"}
	main()

	// Validar que cli.NewRootCmd está disponible
	if root := cli.NewRootCmd(); root == nil {
		t.Fatal("cli.NewRootCmd() retornó nil")
	}
}
