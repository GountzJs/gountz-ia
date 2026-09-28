package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEstimateTokens(t *testing.T) {
	cases := []struct {
		charCount int
		expected  int
	}{
		{0, 0},
		{-5, 0},
		{1, 1},
		{4, 1},
		{5, 2},
		{8, 2},
		{100, 25},
	}

	for _, tc := range cases {
		got := EstimateTokens(tc.charCount)
		if got != tc.expected {
			t.Errorf("EstimateTokens(%d) = %d; want %d", tc.charCount, got, tc.expected)
		}
	}
}

func TestCollector_FindConversationID(t *testing.T) {
	tempDir := t.TempDir()
	brainDir := filepath.Join(tempDir, "brain")
	historyPath := filepath.Join(tempDir, "history.jsonl")

	// Crear archivo history.jsonl sintético
	historyContent := fmt.Sprintf(`{"display":"prompt 1","timestamp":1000,"workspace":"/other/dir","conversationId":"other-conv-1"}
{"display":"prompt 2","timestamp":2000,"workspace":"%s/worktree-1","conversationId":"conv-worktree-1"}
{"display":"prompt 3","timestamp":3000,"workspace":"%s/project","conversationId":"conv-project-3"}
{"display":"prompt 4","timestamp":4000,"workspace":"%s/project","conversationId":"conv-project-4"}
`, tempDir, tempDir, tempDir)

	if err := os.WriteFile(historyPath, []byte(historyContent), 0644); err != nil {
		t.Fatalf("falló al escribir history.jsonl: %v", err)
	}

	collector := NewCollector(
		WithBrainDir(brainDir),
		WithHistoryPath(historyPath),
	)

	// Caso 1: Matching por worktree
	convID, err := collector.FindConversationID("sess1", []string{filepath.Join(tempDir, "worktree-1")})
	if err != nil {
		t.Fatalf("error inesperado en matching worktree: %v", err)
	}
	if convID != "conv-worktree-1" {
		t.Errorf("convID esperado 'conv-worktree-1', obtenido '%s'", convID)
	}

	// Caso 2: Matching por timestamp cercano (2900ms cercano a timestamp 3000)
	targetTime := time.UnixMilli(2900)
	convID, err = collector.FindConversationID("sess2", []string{filepath.Join(tempDir, "project")}, targetTime)
	if err != nil {
		t.Fatalf("error inesperado en matching por timestamp: %v", err)
	}
	if convID != "conv-project-3" {
		t.Errorf("convID esperado 'conv-project-3', obtenido '%s'", convID)
	}

	// Caso 3: Matching sin timestamp debe retornar el más reciente
	convID, err = collector.FindConversationID("sess3", []string{filepath.Join(tempDir, "project")})
	if err != nil {
		t.Fatalf("error inesperado en matching sin timestamp: %v", err)
	}
	if convID != "conv-project-4" {
		t.Errorf("convID esperado 'conv-project-4', obtenido '%s'", convID)
	}

	// Caso 4: Workspace no encontrado
	_, err = collector.FindConversationID("sess4", []string{"/no/existe"})
	if err == nil {
		t.Errorf("se esperaba error para directorio inexistente en historial")
	}

	// Caso 5: Coincidencia directa con transcript existente en brainDir
	directConvDir := filepath.Join(brainDir, "direct-session-id", ".system_generated", "logs")
	if err := os.MkdirAll(directConvDir, 0755); err != nil {
		t.Fatalf("falló al crear directConvDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directConvDir, "transcript.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatalf("falló al crear transcript directo: %v", err)
	}

	convID, err = collector.FindConversationID("direct-session-id", []string{"/no/importa"})
	if err != nil {
		t.Fatalf("error inesperado para ID directo: %v", err)
	}
	if convID != "direct-session-id" {
		t.Errorf("convID esperado 'direct-session-id', obtenido '%s'", convID)
	}
}

func TestCollector_CollectWithSubagents(t *testing.T) {
	tempDir := t.TempDir()
	brainDir := filepath.Join(tempDir, "brain")

	parentConvID := "parent-conv-123"
	subConvID := "subagent-conv-456"

	parentLogsDir := filepath.Join(brainDir, parentConvID, ".system_generated", "logs")
	subLogsDir := filepath.Join(brainDir, subConvID, ".system_generated", "logs")

	if err := os.MkdirAll(parentLogsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(subLogsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Crear transcript del subagente
	subTranscript := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-24T20:00:00Z","content":"Ejecuta los tests unitarios"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-09-24T20:00:05Z","thinking":"Voy a ejecutar go test","tool_calls":[{"name":"run_command","args":{"CommandLine":"go test ./..."}}]}
{"step_index":2,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-09-24T20:00:10Z","content":"PASS ok"}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-09-24T20:00:15Z","content":"Los tests pasaron exitosamente."}
`
	if err := os.WriteFile(filepath.Join(subLogsDir, "transcript.jsonl"), []byte(subTranscript), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Crear transcript de la sesión principal
	subagentArgsJSON := `{"Subagents":[{"Model":"inherit","Prompt":"Ejecuta los tests unitarios","Role":"QA Specialist","TypeName":"self","Workspace":"inherit"}]}`
	subagentResultJSON := fmt.Sprintf("Created the following subagents:\n{\n  \"conversationId\":  \"%s\",\n  \"logAbsoluteUri\":  \"file://...\"\n}", subConvID)

	step1 := TranscriptStep{
		StepIndex: 0,
		Source:    "USER_EXPLICIT",
		Type:      "USER_INPUT",
		Status:    "DONE",
		CreatedAt: "2026-09-24T19:59:50Z",
		Content:   "Por favor valida el código del arnés",
	}
	step2 := TranscriptStep{
		StepIndex: 1,
		Source:    "MODEL",
		Type:      "PLANNER_RESPONSE",
		Status:    "DONE",
		CreatedAt: "2026-09-24T19:59:55Z",
		Thinking:  "Delegaré la tarea a un subagente de QA",
		ToolCalls: []TranscriptToolCall{
			{
				Name: "invoke_subagent",
				Args: json.RawMessage(subagentArgsJSON),
			},
		},
	}
	step3 := TranscriptStep{
		StepIndex: 2,
		Source:    "MODEL",
		Type:      "GENERIC",
		Status:    "DONE",
		CreatedAt: "2026-09-24T20:00:00Z",
		Content:   subagentResultJSON,
	}
	step4 := TranscriptStep{
		StepIndex: 3,
		Source:    "MODEL",
		Type:      "PLANNER_RESPONSE",
		Status:    "DONE",
		CreatedAt: "2026-09-24T20:00:20Z",
		Content:   "He delegado y validado las pruebas con el subagente.",
		ToolCalls: []TranscriptToolCall{
			{
				Name: "view_file",
				Args: json.RawMessage(`{"AbsolutePath":"/some/file.go"}`),
			},
		},
	}

	var parentLines []string
	for _, st := range []TranscriptStep{step1, step2, step3, step4} {
		b, _ := json.Marshal(st)
		parentLines = append(parentLines, string(b))
	}
	parentContent := ""
	for _, l := range parentLines {
		parentContent += l + "\n"
	}
	// Agregar una línea corrupta para validar robustez
	parentContent += "{linea invalida json}\n"

	if err := os.WriteFile(filepath.Join(parentLogsDir, "transcript.jsonl"), []byte(parentContent), 0644); err != nil {
		t.Fatal(err)
	}

	collector := NewCollector(WithBrainDir(brainDir))
	metrics, err := collector.Collect("session-abc", parentConvID)
	if err != nil {
		t.Fatalf("Collector.Collect falló: %v", err)
	}

	if metrics.SessionID != "session-abc" {
		t.Errorf("SessionID esperado 'session-abc', obtenido '%s'", metrics.SessionID)
	}
	if metrics.ConversationID != parentConvID {
		t.Errorf("ConversationID esperado '%s', obtenido '%s'", parentConvID, metrics.ConversationID)
	}
	if metrics.StepsCount != 4 {
		t.Errorf("StepsCount esperado 4, obtenido %d", metrics.StepsCount)
	}
	if metrics.TotalDuration != 30*time.Second {
		t.Errorf("TotalDuration esperado 30s, obtenido %v", metrics.TotalDuration)
	}

	// Validar conteo de herramientas de la sesión principal
	if metrics.ToolCalls["invoke_subagent"] != 1 {
		t.Errorf("conteo invoke_subagent esperado 1, obtenido %d", metrics.ToolCalls["invoke_subagent"])
	}
	if metrics.ToolCalls["view_file"] != 1 {
		t.Errorf("conteo view_file esperado 1, obtenido %d", metrics.ToolCalls["view_file"])
	}

	// Validar tokens
	if metrics.Tokens.PromptTokens == 0 {
		t.Errorf("PromptTokens no debería ser 0")
	}
	if metrics.Tokens.ThinkingTokens == 0 {
		t.Errorf("ThinkingTokens no debería ser 0")
	}
	if metrics.Tokens.CompletionTokens == 0 {
		t.Errorf("CompletionTokens no debería ser 0")
	}
	if metrics.Tokens.TotalTokens != metrics.Tokens.PromptTokens+metrics.Tokens.ThinkingTokens+metrics.Tokens.CompletionTokens {
		t.Errorf("TotalTokens inconsistente: %d != %d + %d + %d",
			metrics.Tokens.TotalTokens, metrics.Tokens.PromptTokens, metrics.Tokens.ThinkingTokens, metrics.Tokens.CompletionTokens)
	}

	// Validar telemetría del subagente
	if len(metrics.Subagents) != 1 {
		t.Fatalf("se esperaba 1 subagente, obtenidos %d", len(metrics.Subagents))
	}
	sub := metrics.Subagents[0]
	if sub.ConversationID != subConvID {
		t.Errorf("ConversationID de subagente esperado '%s', obtenido '%s'", subConvID, sub.ConversationID)
	}
	if sub.Role != "QA Specialist" {
		t.Errorf("Role esperado 'QA Specialist', obtenido '%s'", sub.Role)
	}
	if sub.TypeName != "self" {
		t.Errorf("TypeName esperado 'self', obtenido '%s'", sub.TypeName)
	}
	if sub.Status != "completed" {
		t.Errorf("Status esperado 'completed', obtenido '%s'", sub.Status)
	}
	if sub.StepsCount != 4 {
		t.Errorf("StepsCount de subagente esperado 4, obtenido %d", sub.StepsCount)
	}
	if sub.Duration != 15*time.Second {
		t.Errorf("Duration de subagente esperada 15s, obtenida %v", sub.Duration)
	}
	if sub.ToolCalls["run_command"] != 1 {
		t.Errorf("ToolCalls['run_command'] de subagente esperado 1, obtenido %d", sub.ToolCalls["run_command"])
	}
	if sub.Tokens.TotalTokens == 0 {
		t.Errorf("TotalTokens de subagente no debería ser 0")
	}
}

func TestService_GetMetrics_EndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	brainDir := filepath.Join(tempDir, "brain")
	historyPath := filepath.Join(tempDir, "history.jsonl")
	workDir := filepath.Join(tempDir, "myproject")

	sessID := "sess-xyz"
	convID := "conv-789"

	// 1. Guardar metadata de sesión en .harness/sessions/<id>.json
	harnessDir := filepath.Join(workDir, ".harness", "sessions")
	if err := os.MkdirAll(harnessDir, 0755); err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-1 * time.Minute)
	finished := time.Now()
	metaJSON := fmt.Sprintf(`{
		"id": "%s",
		"working_dir": "%s",
		"worktree_dir": "%s/isolated",
		"started_at": "%s",
		"finished_at": "%s",
		"duration_ms": 60000
	}`, sessID, workDir, workDir, started.Format(time.RFC3339), finished.Format(time.RFC3339))

	if err := os.WriteFile(filepath.Join(harnessDir, sessID+".json"), []byte(metaJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Escribir history.jsonl mapeando el worktree con convID
	histEntry := fmt.Sprintf(`{"display":"inicio","timestamp":%d,"workspace":"%s/isolated","conversationId":"%s"}`+"\n",
		started.UnixMilli(), workDir, convID)
	if err := os.WriteFile(historyPath, []byte(histEntry), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Crear transcript de convID
	tDir := filepath.Join(brainDir, convID, ".system_generated", "logs")
	if err := os.MkdirAll(tDir, 0755); err != nil {
		t.Fatal(err)
	}
	tContent := fmt.Sprintf(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"%s","content":"Hola"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"%s","thinking":"Pensando...","tool_calls":[{"name":"run_command","args":{"CommandLine":"ls"}}]}
`, started.Format(time.RFC3339), finished.Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(tDir, "transcript.jsonl"), []byte(tContent), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(
		WithBrainDir(brainDir),
		WithHistoryPath(historyPath),
	)

	ctx := context.Background()
	res, err := svc.GetMetrics(ctx, sessID, workDir)
	if err != nil {
		t.Fatalf("GetMetrics falló: %v", err)
	}

	if res.SessionID != sessID {
		t.Errorf("SessionID esperado '%s', obtenido '%s'", sessID, res.SessionID)
	}
	if res.ConversationID != convID {
		t.Errorf("ConversationID esperado '%s', obtenido '%s'", convID, res.ConversationID)
	}
	if res.StepsCount != 2 {
		t.Errorf("StepsCount esperado 2, obtenido %d", res.StepsCount)
	}
	if res.ToolCalls["run_command"] != 1 {
		t.Errorf("ToolCalls['run_command'] esperado 1, obtenido %d", res.ToolCalls["run_command"])
	}
}

func TestCollector_MultipleSubagents(t *testing.T) {
	tempDir := t.TempDir()
	brainDir := filepath.Join(tempDir, "brain")

	parentConvID := "parent-multi-123"
	sub1ConvID := "sub-1-abc"
	sub2ConvID := "sub-2-def"

	parentLogsDir := filepath.Join(brainDir, parentConvID, ".system_generated", "logs")
	sub1LogsDir := filepath.Join(brainDir, sub1ConvID, ".system_generated", "logs")
	sub2LogsDir := filepath.Join(brainDir, sub2ConvID, ".system_generated", "logs")

	_ = os.MkdirAll(parentLogsDir, 0755)
	_ = os.MkdirAll(sub1LogsDir, 0755)
	_ = os.MkdirAll(sub2LogsDir, 0755)

	// Transcripts de los 2 subagentes
	_ = os.WriteFile(filepath.Join(sub1LogsDir, "transcript.jsonl"), []byte(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-24T20:00:00Z","content":"sub1 prompt"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-09-24T20:00:05Z","content":"sub1 done","tool_calls":[{"name":"view_file","args":{}}]}
`), 0644)

	_ = os.WriteFile(filepath.Join(sub2LogsDir, "transcript.jsonl"), []byte(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-24T20:00:10Z","content":"sub2 prompt"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-09-24T20:00:15Z","content":"sub2 done","tool_calls":[{"name":"run_command","args":{}}]}
`), 0644)

	// Parent transcript con 2 subagentes invocados simultáneamente
	subagentsArgs := `{"Subagents":[{"Role":"Researcher","TypeName":"research","Prompt":"Investigar docs"},{"Role":"Implementer","TypeName":"self","Prompt":"Codificar feature"}]}`
	subagentsResult := fmt.Sprintf("Created the following subagents:\n[\n  {\"conversationId\": \"%s\"},\n  {\"conversationId\": \"%s\"}\n]", sub1ConvID, sub2ConvID)

	step1 := TranscriptStep{
		StepIndex: 0,
		Source:    "USER_EXPLICIT",
		Type:      "USER_INPUT",
		Status:    "DONE",
		CreatedAt: "2026-09-24T19:59:50Z",
		Content:   "Inicia equipo",
	}
	step2 := TranscriptStep{
		StepIndex: 1,
		Source:    "MODEL",
		Type:      "PLANNER_RESPONSE",
		Status:    "DONE",
		CreatedAt: "2026-09-24T19:59:55Z",
		ToolCalls: []TranscriptToolCall{
			{
				Name: "invoke_subagent",
				Args: json.RawMessage(subagentsArgs),
			},
		},
	}
	step3 := TranscriptStep{
		StepIndex: 2,
		Source:    "MODEL",
		Type:      "GENERIC",
		Status:    "DONE",
		CreatedAt: "2026-09-24T20:00:00Z",
		Content:   subagentsResult,
	}
	step4 := TranscriptStep{
		StepIndex: 3,
		Source:    "MODEL",
		Type:      "PLANNER_RESPONSE",
		Status:    "DONE",
		CreatedAt: "2026-09-24T20:00:20Z",
		Content:   "Listo",
	}

	var parentLines []string
	for _, st := range []TranscriptStep{step1, step2, step3, step4} {
		b, _ := json.Marshal(st)
		parentLines = append(parentLines, string(b))
	}
	parentContent := strings.Join(parentLines, "\n") + "\n"
	_ = os.WriteFile(filepath.Join(parentLogsDir, "transcript.jsonl"), []byte(parentContent), 0644)

	collector := NewCollector(WithBrainDir(brainDir))
	metrics, err := collector.Collect("session-multi", parentConvID)
	if err != nil {
		t.Fatalf("Collector.Collect falló: %v", err)
	}

	if len(metrics.Subagents) != 2 {
		t.Fatalf("se esperaban 2 subagentes, obtenidos %d", len(metrics.Subagents))
	}

	if metrics.Subagents[0].Role != "Researcher" || metrics.Subagents[0].ConversationID != sub1ConvID {
		t.Errorf("Subagente 0 inválido: %+v", metrics.Subagents[0])
	}
	if metrics.Subagents[1].Role != "Implementer" || metrics.Subagents[1].ConversationID != sub2ConvID {
		t.Errorf("Subagente 1 inválido: %+v", metrics.Subagents[1])
	}
	if metrics.Subagents[0].ToolCalls["view_file"] != 1 {
		t.Errorf("Subagente 0 no registró view_file")
	}
	if metrics.Subagents[1].ToolCalls["run_command"] != 1 {
		t.Errorf("Subagente 1 no registró run_command")
	}
}

func TestCollector_Errors(t *testing.T) {
	tempDir := t.TempDir()
	collector := NewCollector(
		WithBrainDir(filepath.Join(tempDir, "brain")),
		WithHistoryPath(filepath.Join(tempDir, "no-history.jsonl")),
	)

	// Historial no existe
	_, err := collector.FindConversationID("any", []string{"/dir"})
	if err == nil {
		t.Errorf("se esperaba error cuando el historial no existe")
	}

	// Transcript no existe
	_, err = collector.Collect("sess", "conv-no-existe")
	if err == nil {
		t.Errorf("se esperaba error cuando transcript no existe")
	}

	// Empty conversation ID
	_, err = collector.Collect("sess", "")
	if err == nil {
		t.Errorf("se esperaba error con conversationID vacío")
	}
}

func TestMetricsService_DirectorySessionMetadata(t *testing.T) {
	tempDir := t.TempDir()
	brainDir := filepath.Join(tempDir, "brain")
	workDir := filepath.Join(tempDir, "workspace")
	historyPath := filepath.Join(tempDir, "history.jsonl")

	sessID := "sess-dir-meta"
	convID := "conv-dir-meta"

	// Guardar metadata en la nueva estructura de directorio: .harness/sessions/<id>/session.json
	sessDir := filepath.Join(workDir, ".harness", "sessions", sessID)
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-2 * time.Minute)
	finished := time.Now()
	metaJSON := fmt.Sprintf(`{
		"id": "%s",
		"working_dir": "%s",
		"worktree_dir": "%s/isolated",
		"started_at": "%s",
		"finished_at": "%s",
		"duration_ms": 120000
	}`, sessID, workDir, workDir, started.Format(time.RFC3339), finished.Format(time.RFC3339))

	if err := os.WriteFile(filepath.Join(sessDir, "session.json"), []byte(metaJSON), 0644); err != nil {
		t.Fatal(err)
	}

	histEntry := fmt.Sprintf(`{"display":"inicio","timestamp":%d,"workspace":"%s/isolated","conversationId":"%s"}`+"\n",
		started.UnixMilli(), workDir, convID)
	if err := os.WriteFile(historyPath, []byte(histEntry), 0644); err != nil {
		t.Fatal(err)
	}

	tDir := filepath.Join(brainDir, convID, ".system_generated", "logs")
	if err := os.MkdirAll(tDir, 0755); err != nil {
		t.Fatal(err)
	}
	tContent := fmt.Sprintf(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"%s","content":"Hola"}
`, started.Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(tDir, "transcript.jsonl"), []byte(tContent), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(
		WithBrainDir(brainDir),
		WithHistoryPath(historyPath),
	)

	ctx := context.Background()
	res, err := svc.GetMetrics(ctx, sessID, workDir)
	if err != nil {
		t.Fatalf("GetMetrics falló: %v", err)
	}

	if res.SessionID != sessID {
		t.Errorf("SessionID esperado '%s', obtenido '%s'", sessID, res.SessionID)
	}
	if res.ConversationID != convID {
		t.Errorf("ConversationID esperado '%s', obtenido '%s'", convID, res.ConversationID)
	}
	if res.TotalDuration != 120*time.Second {
		t.Errorf("TotalDuration esperado 120s, obtenido %v", res.TotalDuration)
	}
}
