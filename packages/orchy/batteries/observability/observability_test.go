package observability

import (
	"context"
	"testing"

	"gz-ia/internal/features/logger"
	"gz-ia/packages/orchy"
)

func TestSessionLogTool_ExecuteSuccess(t *testing.T) {
	tempDir := t.TempDir()
	logSvc := logger.NewService(tempDir)
	tool := NewSessionLogTool(logSvc, WithDefaultSessionID("sess-123"))

	ctx := context.Background()
	dur := int64(150)
	input := map[string]any{
		"action":   "Explorando dependencias del proyecto",
		"stage":    "READ",
		"status":   "OK",
		"role":     "Architect",
		"agent":    "sub-researcher",
		"duration": dur,
	}

	res, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute falló inesperadamente: %v", err)
	}

	out, ok := res.(LogToolOutput)
	if !ok {
		t.Fatalf("tipo de respuesta inválido: %T", res)
	}

	if !out.Success {
		t.Errorf("se esperaba success true")
	}
	if out.SessionID != "sess-123" {
		t.Errorf("sessionID esperado 'sess-123', obtenido '%s'", out.SessionID)
	}
	if out.Action != "Explorando dependencias del proyecto" {
		t.Errorf("action inesperada: %s", out.Action)
	}
	if out.Stage != "READ" {
		t.Errorf("stage esperado 'READ', obtenido '%s'", out.Stage)
	}
	if out.Status == nil || *out.Status != "OK" {
		t.Errorf("status esperado 'OK', obtenido: %v", out.Status)
	}

	// Verificar que el evento se persistió en el logger.Service
	events, err := logSvc.GetEvents(ctx, "sess-123")
	if err != nil {
		t.Fatalf("GetEvents falló: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("se esperaba 1 evento persistido, obtenidos: %d", len(events))
	}
	evt := events[0]
	if evt.Action != "Explorando dependencias del proyecto" || evt.Stage != logger.StageRead {
		t.Errorf("evento persistido incorrecto: %+v", evt)
	}
	if evt.AgentID != "sub-researcher" || evt.Role != "Architect" {
		t.Errorf("metadatos de agente incorrectos: agent=%s, role=%s", evt.AgentID, evt.Role)
	}
	if evt.DurationMs == nil || *evt.DurationMs != 150 {
		t.Errorf("duración incorrecta: %v", evt.DurationMs)
	}
}

func TestSessionLogTool_ValidationErrors(t *testing.T) {
	tempDir := t.TempDir()
	logSvc := logger.NewService(tempDir)
	tool := NewSessionLogTool(logSvc) // sin defaultSessionID
	ctx := context.Background()

	// 1. Falta action
	_, err := tool.Execute(ctx, map[string]any{"session_id": "sess-1"})
	if err == nil || err.Error() != "action is required" {
		t.Errorf("se esperaba error 'action is required', obtenido: %v", err)
	}

	// 2. Falta session_id y no hay default
	_, err = tool.Execute(ctx, map[string]any{"action": "test"})
	if err == nil || err.Error() != "session_id is required" {
		t.Errorf("se esperaba error 'session_id is required', obtenido: %v", err)
	}

	// 3. Stage inválido
	toolWithID := NewSessionLogTool(logSvc, WithDefaultSessionID("sess-1"))
	_, err = toolWithID.Execute(ctx, map[string]any{
		"action": "test",
		"stage":  "INVALID_STAGE",
	})
	if err == nil {
		t.Error("se esperaba error con stage inválido")
	}

	// 4. Status inválido
	_, err = toolWithID.Execute(ctx, map[string]any{
		"action": "test",
		"status": "INVALID_STATUS",
	})
	if err == nil {
		t.Error("se esperaba error con status inválido")
	}
}

func TestSessionLogTool_DefaultStageAndAgent(t *testing.T) {
	tempDir := t.TempDir()
	logSvc := logger.NewService(tempDir)
	tool := NewSessionLogTool(logSvc, WithDefaultSessionID("sess-default"))
	ctx := context.Background()

	res, err := tool.Execute(ctx, map[string]any{
		"action": "Acción sin stage ni agent explícito",
	})
	if err != nil {
		t.Fatalf("Execute falló: %v", err)
	}

	out, ok := res.(LogToolOutput)
	if !ok || out.Stage != string(logger.StagePending) {
		t.Errorf("stage por defecto debería ser PENDING, obtenido: %v", out.Stage)
	}

	events, _ := logSvc.GetEvents(ctx, "sess-default")
	if len(events) != 1 || events[0].AgentID != "orchestrator" {
		t.Errorf("agente por defecto debería ser 'orchestrator', obtenido: %+v", events)
	}
}

func TestObservabilityPlugin_BootAndKernelIntegration(t *testing.T) {
	tempDir := t.TempDir()
	logSvc := logger.NewService(tempDir)
	plugin := NewObservabilityPlugin(tempDir,
		WithLogger(logSvc),
		WithSessionID("sess-kernel-test"),
	)

	kernel := orchy.NewKernel()
	if err := kernel.Use(plugin); err != nil {
		t.Fatalf("kernel.Use falló: %v", err)
	}

	ctx := context.Background()
	if err := kernel.Boot(ctx); err != nil {
		t.Fatalf("kernel.Boot falló: %v", err)
	}
	defer func() { _ = kernel.Shutdown(ctx) }()

	// Verificar que la herramienta session_log está registrada en el catálogo
	toolProxy, exists := kernel.GetContext().Tools().Get("session_log")
	if !exists {
		t.Fatalf("herramienta session_log no encontrada en el kernel")
	}

	if toolProxy.Name() != "session_log" {
		t.Errorf("nombre de herramienta inesperado: %s", toolProxy.Name())
	}

	// Ejecutar la herramienta a través del proxy del kernel
	res, err := toolProxy.Execute(ctx, map[string]any{
		"action": "Evento emitido vía kernel Orchy",
		"stage":  "FINISH",
		"status": "OK",
	})
	if err != nil {
		t.Fatalf("ejecución vía proxy falló: %v", err)
	}

	out, ok := res.(LogToolOutput)
	if !ok || !out.Success {
		t.Errorf("resultado inesperado: %+v", res)
	}

	events, err := logSvc.GetEvents(ctx, "sess-kernel-test")
	if err != nil || len(events) != 1 {
		t.Fatalf("se esperaba 1 evento en logSvc, obtenidos: %d (err: %v)", len(events), err)
	}
	if events[0].Action != "Evento emitido vía kernel Orchy" {
		t.Errorf("acción inesperada: %s", events[0].Action)
	}
}
