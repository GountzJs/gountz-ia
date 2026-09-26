package tui

import (
	"testing"
)

func TestGetIcons(t *testing.T) {
	icons := GetIcons()

	if icons.Robot != "›" {
		t.Errorf("Robot esperado '›', obtenido '%s'", icons.Robot)
	}
	if icons.Check != "✓" {
		t.Errorf("Check esperado '✓', obtenido '%s'", icons.Check)
	}
	if icons.Cross != "✕" {
		t.Errorf("Cross esperado '✕', obtenido '%s'", icons.Cross)
	}
	if icons.Sparkle != "✧" {
		t.Errorf("Sparkle esperado '✧', obtenido '%s'", icons.Sparkle)
	}
	if icons.Exit != "←" {
		t.Errorf("Exit esperado '←', obtenido '%s'", icons.Exit)
	}
}

func TestColorsAndStyles(t *testing.T) {
	if string(ColorPrimary) != "#3D6FD4" {
		t.Errorf("ColorPrimary inesperado: %s", ColorPrimary)
	}
	if string(ColorSuccess) != "#06D6A0" {
		t.Errorf("ColorSuccess inesperado: %s", ColorSuccess)
	}

	renderedTitle := StyleTitle.Render("Test Title")
	if len(renderedTitle) == 0 {
		t.Error("StyleTitle.Render produjo una cadena vacía")
	}

	renderedSuccess := StyleSuccess.Render("Success")
	if len(renderedSuccess) == 0 {
		t.Error("StyleSuccess.Render produjo una cadena vacía")
	}
}

func TestCustomHuhTheme(t *testing.T) {
	theme := CustomHuhTheme()
	if theme == nil {
		t.Fatal("CustomHuhTheme() retornó nil")
	}
}
