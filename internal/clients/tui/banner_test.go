package tui

import (
	"strings"
	"testing"
)

func TestRenderFastfetchBanner(t *testing.T) {
	banner := RenderFastfetchBanner()

	if len(banner) == 0 {
		t.Fatal("el banner no debe estar vacío")
	}

	if !strings.Contains(banner, "GountzJs") {
		t.Errorf("el banner debe contener 'GountzJs': %s", banner)
	}

	if !strings.Contains(banner, "Harness") {
		t.Errorf("el banner debe contener 'Harness': %s", banner)
	}

	if !strings.Contains(banner, "Arquitectura") {
		t.Errorf("el banner debe contener 'Arquitectura': %s", banner)
	}

	if !strings.Contains(banner, "Estado") {
		t.Errorf("el banner debe contener 'Estado': %s", banner)
	}
}
