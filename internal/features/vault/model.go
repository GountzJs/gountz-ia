package vault

import (
	"fmt"
	"strconv"
	"time"
)

// Vault representa la estructura de almacenamiento persistente y seguro de secretos y variables de entorno.
type Vault struct {
	Version   int               `json:"version"`
	Env       map[string]string `json:"env"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// EnvStatus representa el estado visible de una variable para inspección, auditoría y recomendaciones seguras.
type EnvStatus struct {
	Key            string   `json:"key"`
	Exists         bool     `json:"exists"`          // Existe en vault o en el entorno del sistema
	InVault        bool     `json:"in_vault"`        // Configurada en el vault local del proyecto
	InSystem       bool     `json:"in_system"`       // Configurada en las variables de entorno del SO
	MaskedValue    string   `json:"masked_value"`    // Valor enmascarado para visualización segura
	Length         int      `json:"length"`          // Longitud del secreto
	RecommendedFor []string `json:"recommended_for"` // Proveedores o perfiles que la utilizan
	Description    string   `json:"description"`     // Breve descripción de la variable
}

// Recommendation define metadatos de variables conocidas comúnmente requeridas por agentes o herramientas.
type Recommendation struct {
	Key            string
	RecommendedFor []string
	Description    string
}

// DefaultRecommendations contiene las recomendaciones base para los drivers soportados por gz-ia.
var DefaultRecommendations = []Recommendation{
	{
		Key:            "ANTHROPIC_API_KEY",
		RecommendedFor: []string{"claude", "Anthropic Claude Code"},
		Description:    "Clave de autenticación oficial para Claude Code / Anthropic",
	},
	{
		Key:            "GEMINI_API_KEY",
		RecommendedFor: []string{"agy", "Google Antigravity"},
		Description:    "Clave de API para Google Gemini y modelos Antigravity",
	},
	{
		Key:            "OPENAI_API_KEY",
		RecommendedFor: []string{"opencode", "OpenCode AI"},
		Description:    "Clave de API para modelos OpenAI y OpenCode",
	},
	{
		Key:            "GITHUB_TOKEN",
		RecommendedFor: []string{"github", "herramientas CI/Git"},
		Description:    "Token de acceso para GitHub CLI, Releases y workflows",
	},
}

// MaskSecret ofusca un secreto para que nunca se imprima en texto plano en la consola o logs.
func MaskSecret(val string) string {
	if val == "" {
		return "(vacía)"
	}
	n := len(val)
	if n <= 4 {
		return fmt.Sprintf("**** (%d car.)", n)
	}
	if n <= 8 {
		return fmt.Sprintf("%s***%s (%d car.)", string(val[0]), string(val[n-1]), n)
	}
	prefix := val[:3]
	suffix := val[n-2:]
	return fmt.Sprintf("%s...%s (%d caracteres)", prefix, suffix, n)
}

// MaskSecretDescription retorna una versión compacta del valor enmascarado.
func MaskSecretCompact(val string) string {
	if val == "" {
		return ""
	}
	n := len(val)
	if n <= 6 {
		return "******"
	}
	return val[:2] + "..." + val[n-2:] + " (" + strconv.Itoa(n) + ")"
}
