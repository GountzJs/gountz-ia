package tui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// IconSet define los glifos usados en la interfaz.
type IconSet struct {
	Robot     string
	GitBranch string
	Key       string
	Sync      string
	Doctor    string
	Config    string
	Exit      string
	Check     string
	Cross     string
	Warning   string
	Rocket    string
	Folder    string
	Search    string
	Back      string
	Sparkle   string
	Terminal  string
	User      string
	Worktree  string
	Box       string
	Arrow     string
	Palette   string
	Bolt      string
	Shield    string
	Lock      string
	Plan      string
}

// GetIcons retorna el set de glifos tipográficos minimalistas y universales (estilo gh / cargo / pnpm).
func GetIcons() IconSet {
	return IconSet{
		Robot:     "›",
		GitBranch: "⌥",
		Key:       "∗",
		Sync:      "↻",
		Doctor:    "✚",
		Config:    "⚙",
		Exit:      "←",
		Check:     "✓",
		Cross:     "✕",
		Warning:   "▲",
		Rocket:    "❯❯",
		Folder:    "▪",
		Search:    "•",
		Back:      "‹",
		Sparkle:   "✧",
		Terminal:  "❯",
		User:      "•",
		Worktree:  "├─",
		Box:       "◆",
		Arrow:     "❯",
		Palette:   "●",
		Bolt:      "⚡",
		Shield:    "•",
		Lock:      "•",
		Plan:      "•",
	}
}

// Paleta de colores Lip Gloss (estilo Cyber/Fastfetch)
var (
	ColorPrimary   = lipgloss.Color("#3D6FD4") // Azul luna (color del avatar)
	ColorSecondary = lipgloss.Color("#7BA7E8") // Azul cielo claro
	ColorAccent    = lipgloss.Color("#A8C4F0") // Azul hielo
	ColorSuccess   = lipgloss.Color("#06D6A0") // Verde esmeralda
	ColorWarning   = lipgloss.Color("#FFBE0B") // Ámbar
	ColorDanger    = lipgloss.Color("#EF476F") // Rojo coral
	ColorMuted     = lipgloss.Color("#5C7A9E") // Azul gris apagado
	ColorSubtle    = lipgloss.Color("#2A3A50") // Azul marino oscuro
	ColorWhite     = lipgloss.Color("#EEF2F7") // Blanco azulado frío

	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary)

	StyleKey = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary)

	StyleValue = lipgloss.NewStyle().
			Foreground(ColorWhite)

	StyleMuted = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	StyleDanger = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	StyleBadge = lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(ColorWhite).
			Bold(true).
			Padding(0, 1)

	StyleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1)
)

// CustomHuhTheme retorna el tema visual adaptado para la suite Huh.
func CustomHuhTheme() *huh.Theme {
	return huh.ThemeCatppuccin()
}
