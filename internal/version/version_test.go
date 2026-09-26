package version

import (
	"testing"
)

func TestVersionDefaults(t *testing.T) {
	if Version == "" {
		t.Fatal("Version no debe estar vacía")
	}
	if Commit == "" {
		t.Fatal("Commit no debe estar vacío")
	}
	if Date == "" {
		t.Fatal("Date no debe estar vacía")
	}
}
