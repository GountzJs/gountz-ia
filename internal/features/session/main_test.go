package session

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	origLookPath := lookPath
	// Por defecto en las pruebas unitarias, simular que agy está disponible en el PATH
	lookPath = func(file string) (string, error) {
		if file == "agy" {
			return "/mock/bin/agy", nil
		}
		return origLookPath(file)
	}
	code := m.Run()
	lookPath = origLookPath
	os.Exit(code)
}
