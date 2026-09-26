package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	uuidRegex   = regexp.MustCompile(`[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}`)
	convIDRegex = regexp.MustCompile(`"conversationId"\s*:\s*"([^"]+)"`)
)

// TranscriptStep modela una línea individual dentro del archivo transcript.jsonl.
type TranscriptStep struct {
	StepIndex int                  `json:"step_index"`
	Source    string               `json:"source"`
	Type      string               `json:"type"`
	Status    string               `json:"status"`
	CreatedAt string               `json:"created_at"`
	Content   string               `json:"content"`
	Thinking  string               `json:"thinking"`
	ToolCalls []TranscriptToolCall `json:"tool_calls"`
}

// TranscriptToolCall modela una llamada a herramienta dentro de un paso del transcript.
type TranscriptToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// HistoryEntry modela una línea dentro de history.jsonl de Antigravity.
type HistoryEntry struct {
	Display        string `json:"display"`
	Timestamp      int64  `json:"timestamp"`
	Workspace      string `json:"workspace"`
	ConversationID string `json:"conversationId"`
}

// SubagentArg representa los parámetros individuales de configuración en invoke_subagent.
type SubagentArg struct {
	Model     string `json:"Model"`
	Prompt    string `json:"Prompt"`
	Role      string `json:"Role"`
	TypeName  string `json:"TypeName"`
	Workspace string `json:"Workspace"`
}

// Collector gestiona la localización y análisis de trazas de telemetría de Antigravity.
type Collector struct {
	brainDir    string
	historyPath string
}

// Option permite personalizar los directorios y rutas del Collector y Service.
type Option func(*options)

type options struct {
	brainDir    string
	historyPath string
}

func defaultOptions() *options {
	return &options{
		brainDir:    DefaultBrainDir(),
		historyPath: DefaultHistoryPath(),
	}
}

// WithBrainDir inyecta la ruta al directorio base de brain de Antigravity.
func WithBrainDir(dir string) Option {
	return func(o *options) {
		o.brainDir = dir
	}
}

// WithHistoryPath inyecta la ruta al archivo history.jsonl de Antigravity.
func WithHistoryPath(path string) Option {
	return func(o *options) {
		o.historyPath = path
	}
}

// DefaultBrainDir resuelve la ruta estándar a ~/.gemini/antigravity-cli/brain de forma portable.
func DefaultBrainDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "brain")
}

// DefaultHistoryPath resuelve la ruta estándar a ~/.gemini/antigravity-cli/history.jsonl de forma portable.
func DefaultHistoryPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "history.jsonl")
}

// NewCollector crea una nueva instancia de Collector con opciones configurables.
func NewCollector(opts ...Option) *Collector {
	cfg := defaultOptions()
	for _, opt := range opts {
		opt(cfg)
	}
	return &Collector{
		brainDir:    cfg.brainDir,
		historyPath: cfg.historyPath,
	}
}

// BrainDir retorna el directorio brain configurado.
func (c *Collector) BrainDir() string {
	return c.brainDir
}

// HistoryPath retorna la ruta a history.jsonl configurada.
func (c *Collector) HistoryPath() string {
	return c.historyPath
}

// EstimateTokens calcula la cantidad aproximada de tokens en base a la longitud en caracteres.
// Utiliza la heurística estándar de la industria de ~4 caracteres por token.
func EstimateTokens(charCount int) int {
	if charCount <= 0 {
		return 0
	}
	return (charCount + 3) / 4
}

// ResolveTranscriptPath localiza la ruta al transcript.jsonl para un conversationId dado.
func (c *Collector) ResolveTranscriptPath(convID string) string {
	if convID == "" || c.brainDir == "" {
		return ""
	}
	primary := filepath.Join(c.brainDir, convID, ".system_generated", "logs", "transcript.jsonl")
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	fallback := filepath.Join(c.brainDir, convID, "transcript.jsonl")
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}
	return primary
}

// FindConversationID localiza el ID de conversación asociado a la sesión comparando los directorios de trabajo.
func (c *Collector) FindConversationID(sessionID string, targetDirs []string, targetTime ...time.Time) (string, error) {
	// 1. Verificar si sessionID corresponde directamente a un transcript existente en brainDir
	if sessionID != "" {
		tPath := c.ResolveTranscriptPath(sessionID)
		if _, err := os.Stat(tPath); err == nil {
			return sessionID, nil
		}
	}

	// 2. Si no se especificó historyPath o no existe, verificar directorios objetivo
	if c.historyPath == "" {
		return "", fmt.Errorf("historyPath no configurado")
	}

	file, err := os.Open(c.historyPath)
	if err != nil {
		if os.IsNotExist(err) && sessionID != "" {
			return "", fmt.Errorf("historial de Antigravity no encontrado en '%s'", c.historyPath)
		}
		return "", fmt.Errorf("error al abrir historial de Antigravity: %w", err)
	}
	defer file.Close()

	var cleanTargets []string
	for _, td := range targetDirs {
		if td != "" {
			cleanTargets = append(cleanTargets, filepath.Clean(td))
		}
	}

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var matchedEntries []HistoryEntry
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.ConversationID == "" {
			continue
		}

		cleanWs := filepath.Clean(entry.Workspace)
		for _, target := range cleanTargets {
			if cleanWs == target {
				matchedEntries = append(matchedEntries, entry)
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error al escanear historial de Antigravity: %w", err)
	}

	if len(matchedEntries) == 0 {
		return "", fmt.Errorf("no se encontró conversationId en el historial para los directorios %v", cleanTargets)
	}

	// Si se suministró una marca de tiempo de referencia, buscar la entrada más cercana temporalmente
	if len(targetTime) > 0 && !targetTime[0].IsZero() {
		targetMillis := targetTime[0].UnixMilli()
		bestIdx := 0
		var bestDiff int64 = -1

		for i, entry := range matchedEntries {
			diff := entry.Timestamp - targetMillis
			if diff < 0 {
				diff = -diff
			}
			if bestDiff < 0 || diff < bestDiff {
				bestDiff = diff
				bestIdx = i
			}
		}
		return matchedEntries[bestIdx].ConversationID, nil
	}

	// Por defecto, retornar la entrada más reciente
	return matchedEntries[len(matchedEntries)-1].ConversationID, nil
}

// ParseTranscriptSteps lee un archivo transcript.jsonl y retorna todos los pasos válidos.
func (c *Collector) ParseTranscriptSteps(filePath string) ([]TranscriptStep, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error al abrir archivo de transcript '%s': %w", filePath, err)
	}
	defer file.Close()

	var steps []TranscriptStep
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 20*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var step TranscriptStep
		if err := json.Unmarshal([]byte(line), &step); err != nil {
			continue
		}
		steps = append(steps, step)
	}

	if err := scanner.Err(); err != nil {
		return steps, fmt.Errorf("error al leer archivo de transcript: %w", err)
	}

	return steps, nil
}

// ParseSubagentArgs deserializa los argumentos pasados a la herramienta invoke_subagent.
func ParseSubagentArgs(raw json.RawMessage) ([]SubagentArg, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	// Puede venir como objeto que contiene campo Subagents: {"Subagents": [...]}
	var wrapper struct {
		Subagents json.RawMessage `json:"Subagents"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && len(wrapper.Subagents) > 0 {
		// Intentar como array de objetos
		var list []SubagentArg
		if err := json.Unmarshal(wrapper.Subagents, &list); err == nil {
			return list, nil
		}
		// Puede ser un string JSON escapado
		var str string
		if err := json.Unmarshal(wrapper.Subagents, &str); err == nil {
			if err2 := json.Unmarshal([]byte(str), &list); err2 == nil {
				return list, nil
			}
		}
	}

	// O puede ser directamente []SubagentArg
	var list []SubagentArg
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}

	return nil, fmt.Errorf("no se pudo deserializar Subagents de args")
}

// parseTime intenta parsear marcas de tiempo en formato RFC3339 o RFC3339Nano.
func parseTime(ts string) (time.Time, bool) {
	if ts == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// extractSubagentIDs busca IDs de conversación en el contenido de pasos de resultado de invoke_subagent.
func extractSubagentIDs(content string) []string {
	var ids []string
	seen := make(map[string]bool)

	// Buscar primero por "conversationId": "..."
	matches := convIDRegex.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}

	// Si no encontró nada, buscar patrones de UUID
	if len(ids) == 0 {
		uuidMatches := uuidRegex.FindAllString(content, -1)
		for _, u := range uuidMatches {
			if !seen[u] {
				seen[u] = true
				ids = append(ids, u)
			}
		}
	}

	return ids
}

// CollectFromTranscript analiza las líneas de un transcript y extrae la telemetría completa.
func (c *Collector) CollectFromTranscript(convID string, steps []TranscriptStep) (TokenUsage, map[string]int, time.Time, *time.Time, time.Duration, []SubagentMetrics) {
	var tokens TokenUsage
	toolCalls := make(map[string]int)
	var startedAt time.Time
	var finishedAt *time.Time
	var duration time.Duration
	var subagents []SubagentMetrics

	if len(steps) == 0 {
		return tokens, toolCalls, startedAt, finishedAt, duration, subagents
	}

	// Marcas de tiempo de inicio y fin
	for _, step := range steps {
		if t, ok := parseTime(step.CreatedAt); ok {
			if startedAt.IsZero() {
				startedAt = t
			}
			last := t
			finishedAt = &last
		}
	}

	if finishedAt != nil && !startedAt.IsZero() {
		duration = finishedAt.Sub(startedAt)
		if duration < 0 {
			duration = 0
		}
	}

	// Procesar pasos
	for i := 0; i < len(steps); i++ {
		step := steps[i]

		// 1. Thinking tokens
		if step.Thinking != "" {
			tokens.ThinkingTokens += EstimateTokens(len(step.Thinking))
		}

		// 2. Prompt tokens
		if step.Type == "USER_INPUT" || step.Source == "USER_EXPLICIT" || step.Source != "MODEL" || step.Type == "GENERIC" {
			if step.Content != "" {
				tokens.PromptTokens += EstimateTokens(len(step.Content))
			}
		}

		// 3. Completion tokens
		if step.Source == "MODEL" && step.Type == "PLANNER_RESPONSE" {
			if step.Content != "" {
				tokens.CompletionTokens += EstimateTokens(len(step.Content))
			}
		}

		// 4. Herramientas y llamadas
		for _, tc := range step.ToolCalls {
			toolCalls[tc.Name]++
			// Argumentos de herramienta generados por el modelo computan en Completion
			tokens.CompletionTokens += EstimateTokens(len(tc.Name) + len(tc.Args))

			// Detectar invoke_subagent
			if tc.Name == "invoke_subagent" {
				subArgs, _ := ParseSubagentArgs(tc.Args)

				// Buscar conversación en los pasos posteriores
				var subConvIDs []string
				for j := i + 1; j < len(steps) && j <= i+3; j++ {
					nextStep := steps[j]
					if nextStep.Type == "PLANNER_RESPONSE" && nextStep.StepIndex > step.StepIndex+1 {
						break
					}
					found := extractSubagentIDs(nextStep.Content)
					if len(found) > 0 {
						subConvIDs = append(subConvIDs, found...)
						break
					}
				}

				for idx, arg := range subArgs {
					var subID string
					if idx < len(subConvIDs) {
						subID = subConvIDs[idx]
					}

					subMetric := SubagentMetrics{
						ConversationID: subID,
						Role:           arg.Role,
						TypeName:       arg.TypeName,
						Prompt:         arg.Prompt,
						ToolCalls:      make(map[string]int),
						Status:         "completed",
					}

					if subID != "" {
						c.populateSubagent(&subMetric)
					} else {
						subMetric.Status = "unknown"
					}

					subagents = append(subagents, subMetric)
				}
			}
		}
	}

	tokens.TotalTokens = tokens.PromptTokens + tokens.ThinkingTokens + tokens.CompletionTokens
	return tokens, toolCalls, startedAt, finishedAt, duration, subagents
}

// populateSubagent analiza recursivamente el transcript de un subagente para llenar sus métricas.
func (c *Collector) populateSubagent(sub *SubagentMetrics) {
	if sub.ConversationID == "" {
		return
	}

	tPath := c.ResolveTranscriptPath(sub.ConversationID)
	steps, err := c.ParseTranscriptSteps(tPath)
	if err != nil || len(steps) == 0 {
		sub.Status = "pending"
		return
	}

	tokens, tools, start, finish, dur, nestedSubagents := c.CollectFromTranscript(sub.ConversationID, steps)
	sub.StepsCount = len(steps)
	sub.Tokens = tokens
	sub.ToolCalls = tools
	sub.StartedAt = start
	sub.FinishedAt = finish
	sub.Duration = dur

	// Determinar estado final del subagente
	hasError := false
	for _, step := range steps {
		if step.Status == "ERROR" {
			hasError = true
			break
		}
	}
	if hasError {
		sub.Status = "failed"
	} else {
		sub.Status = "completed"
	}

	// Si el subagente invocó a su vez subagentes anidados, agregar sus herramientas y tokens
	for _, nested := range nestedSubagents {
		for tool, count := range nested.ToolCalls {
			sub.ToolCalls[tool] += count
		}
	}
}

// Collect analiza una sesión por su ID de conversación y produce sus SessionMetrics.
func (c *Collector) Collect(sessionID, conversationID string) (*SessionMetrics, error) {
	if conversationID == "" {
		return nil, fmt.Errorf("conversationID no puede estar vacío")
	}

	tPath := c.ResolveTranscriptPath(conversationID)
	steps, err := c.ParseTranscriptSteps(tPath)
	if err != nil {
		return nil, err
	}

	tokens, tools, start, finish, dur, subagents := c.CollectFromTranscript(conversationID, steps)

	metrics := &SessionMetrics{
		SessionID:      sessionID,
		ConversationID: conversationID,
		TotalDuration:  dur,
		StartedAt:      start,
		FinishedAt:     finish,
		StepsCount:     len(steps),
		Tokens:         tokens,
		ToolCalls:      tools,
		Subagents:      subagents,
	}

	return metrics, nil
}
