package tui

import (
	"os"
	"testing"

	"gz-ia/internal/features/session"
)

func TestMain(m *testing.M) {
	// Por defecto en las pruebas de TUI, agy está disponible para permitir selección y lanzamiento
	restore := session.SetLookPathForTesting(func(file string) (string, error) {
		if file == "agy" {
			return "/mock/bin/agy", nil
		}
		return "", os.ErrNotExist
	})
	code := m.Run()
	restore()
	os.Exit(code)
}
