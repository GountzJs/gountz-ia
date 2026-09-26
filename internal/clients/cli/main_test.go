package cli

import (
	"os"
	"testing"

	"gz-ia/internal/features/session"
)

func TestMain(m *testing.M) {
	// Por defecto en las pruebas de CLI, agy está disponible para permitir tests de chat y sesión
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
