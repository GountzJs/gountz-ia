package tui

import (
	"fmt"
	"gz-ia/internal/version"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Arte Braille vectorial generado directamente desde el avatar real de GountzJs (estilo Fastfetch)
var avatarBrailleLines = []string{
	"⠀⠀⣀⣀⣀⣠⠤⠤⠤⠖⠒⠒⠊⠉⠉⠓⢄⡀⠀⠀⠀⠀⠀⠀",
	"⡞⠫⣅⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠉⠲⣄⠀⠀⠀⠀",
	"⡇⠀⠈⠓⢦⡀⠀⠀⠀⠀⢀⣀⣀⣀⣤⣤⣤⣶⣶⡒⠛⡆⠀⠀",
	"⡇⠀⠀⠀⠀⠙⡷⠚⣻⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⡇⠀⠀",
	"⡇⠻⣆⠀⠀⠀⡇⣼⣿⣿⣿⠿⠿⢿⡿⠟⢉⣉⡁⠉⠉⡇⠀⠀",
	"⡇⢀⣽⠇⠀⠀⡁⠉⠁⡤⣴⣶⣶⠀⣠⡀⠻⠿⠿⠏⢠⣇⠔⠀",
	"⡇⠛⢁⡀⠀⠀⡇⣿⣆⠐⠚⠛⠉⣠⣀⣠⣤⢰⣆⠀⠀⠈⠀⡀",
	"⢇⠀⠀⠙⠷⠀⡇⠸⣿⣿⣿⣿⣄⡈⣥⣤⣠⣾⣇⢓⣄⠀⠘⠃",
	"⠀⠉⠢⣀⠀⠀⡇⠀⠙⢿⣿⣿⣿⣿⣮⣷⡿⠿⠛⡠⠄⠀⠀⠀",
	"⠀⠀⠀⠀⠑⠤⣇⣀⣀⡤⠤⠭⠭⠝⠒⠒⠒⠊⠉⠀⠀⠀⠀⠀",
}

// RenderFastfetchBanner genera el banner visual de bienvenida inspirado en fastfetch/neofetch.
func RenderFastfetchBanner() string {
	icons := GetIcons()

	// 1. Logo Braille Vectorial en gradiente de azules del avatar
	brailleGradients := []lipgloss.Color{
		lipgloss.Color("#2952A3"), // Azul marino
		lipgloss.Color("#3563BF"), // Azul cobalto
		lipgloss.Color("#3D6FD4"), // Azul avatar base
		lipgloss.Color("#4F82DE"), // Azul medio
		lipgloss.Color("#5A8FE0"), // Azul claro
		lipgloss.Color("#6C9EE5"), // Azul cielo
		lipgloss.Color("#7BA7E8"), // Azul brillante
		lipgloss.Color("#5A8FE0"), // Azul claro
		lipgloss.Color("#3D6FD4"), // Azul avatar base
		lipgloss.Color("#2952A3"), // Azul marino
	}

	var styledLogoLines []string
	for i, line := range avatarBrailleLines {
		color := brailleGradients[i%len(brailleGradients)]
		styledLogoLines = append(styledLogoLines, lipgloss.NewStyle().Foreground(color).Bold(true).Render(line))
	}

	// Firma de la marca personal GountzJs
	styledLogoLines = append(styledLogoLines, lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render("      GountzJs      "))

	logoBox := lipgloss.NewStyle().
		Padding(0, 1, 0, 1).
		Align(lipgloss.Center).
		Render(strings.Join(styledLogoLines, "\n"))

	// 2. Información del Entorno y Sistema (Columna Derecha)
	currentUser := "developer"
	if u, err := user.Current(); err == nil && u.Username != "" {
		currentUser = u.Username
	} else if u := os.Getenv("USER"); u != "" {
		currentUser = u
	}

	cwd, _ := os.Getwd()
	repoName := filepath.Base(cwd)
	if repoName == "." || repoName == "/" {
		repoName = "gz-ia"
	}

	headerUser := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render(currentUser)
	headerAt := lipgloss.NewStyle().Foreground(ColorMuted).Render("@")
	headerHost := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(repoName)
	titleLine := fmt.Sprintf("%s%s%s", headerUser, headerAt, headerHost)

	divider := lipgloss.NewStyle().
		Foreground(ColorSubtle).
		Render(strings.Repeat("─", 40))

	// Generar filas Fastfetch
	renderRow := func(icon, key, val string) string {
		k := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render(fmt.Sprintf("%s %-12s", icon, key))
		sep := lipgloss.NewStyle().Foreground(ColorSubtle).Render("❯")
		v := lipgloss.NewStyle().Foreground(ColorWhite).Render(val)
		return fmt.Sprintf("%s %s %s", k, sep, v)
	}

	statusStr := lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("Listo (" + icons.Check + ")")

	infoLines := []string{
		titleLine,
		divider,
		renderRow(icons.Terminal, "Harness", "gz-ia (core)"),
		renderRow(icons.Sparkle, "Versión", "v"+version.Version),
		renderRow(icons.GitBranch, "Workspace", repoName),
		renderRow(icons.Config, "Arquitectura", "Desacoplada / Modular"),
		renderRow(icons.Box, "Runtime", runtime.Version()),
		renderRow(icons.Doctor, "Modo", "Standalone / TUI"),
		renderRow(icons.Check, "Estado", statusStr),
		"",
	}

	// Paleta de colores Fastfetch
	paletteColors := []lipgloss.Color{
		lipgloss.Color("#1B429B"),
		lipgloss.Color("#2F6EE8"),
		lipgloss.Color("#5B8EF2"),
		lipgloss.Color("#7BA7E8"),
		lipgloss.Color("#A8C4F0"),
		lipgloss.Color("#06D6A0"),
		lipgloss.Color("#FFBE0B"),
		lipgloss.Color("#EEF2F7"),
	}
	var dots []string
	for _, pc := range paletteColors {
		dots = append(dots, lipgloss.NewStyle().Foreground(pc).Render(icons.Palette))
	}
	infoLines = append(infoLines, strings.Join(dots, " "))

	infoBox := lipgloss.NewStyle().
		Padding(0, 1).
		Render(strings.Join(infoLines, "\n"))

	// 3. Unión horizontal y encuadre con borde redondeado
	joined := lipgloss.JoinHorizontal(lipgloss.Top, logoBox, infoBox)

	bannerContainer := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		MarginBottom(1).
		Render(joined)

	return bannerContainer
}
