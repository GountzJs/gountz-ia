package logger_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gz-ia/internal/features/logger"
)

func TestSerializationStatusNullOKFailed(t *testing.T) {
	tempDir := t.TempDir()
	svc := logger.NewService(tempDir)
	ctx := context.Background()
	sessionID := "test-sess-ser"

	// 1. Status: null
	evtNull := &logger.Event{
		SessionID: sessionID,
		AgentID:   "orch-1",
		Role:      "orchestrator",
		Action:    "Leyendo contexto inicial",
		Stage:     logger.StageRead,
		Status:    nil,
	}
	if err := svc.Emit(ctx, evtNull); err != nil {
		t.Fatalf("Emit evtNull falló: %v", err)
	}

	// 2. Status: "OK"
	statusOK := logger.StatusOK
	evtOK := &logger.Event{
		SessionID: sessionID,
		AgentID:   "orch-1",
		Role:      "orchestrator",
		Action:    "Acción completada exitosamente",
		Stage:     logger.StageFinish,
		Status:    &statusOK,
	}
	if err := svc.Emit(ctx, evtOK); err != nil {
		t.Fatalf("Emit evtOK falló: %v", err)
	}

	// 3. Status: "FAILED"
	statusFailed := logger.StatusFailed
	evtFailed := &logger.Event{
		SessionID: sessionID,
		AgentID:   "orch-1",
		Role:      "orchestrator",
		Action:    "Acción con error",
		Stage:     logger.StageFinish,
		Status:    &statusFailed,
		Error:     "fallo simulado",
	}
	if err := svc.Emit(ctx, evtFailed); err != nil {
		t.Fatalf("Emit evtFailed falló: %v", err)
	}

	// Verificar lectura directa del archivo para comprobar serialización JSON cruda
	eventFile := filepath.Join(tempDir, ".harness", "sessions", sessionID, "events.jsonl")
	content, err := os.ReadFile(eventFile)
	if err != nil {
		t.Fatalf("No se pudo leer archivo de eventos: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 3 {
		t.Fatalf("Se esperaban 3 líneas de eventos, se obtuvieron: %d", len(lines))
	}

	// Verificar línea 1: "status":null
	if !strings.Contains(lines[0], `"status":null`) {
		t.Errorf("Línea 1 debería contener '\"status\":null', obtenido: %s", lines[0])
	}

	// Verificar línea 2: "status":"OK"
	if !strings.Contains(lines[1], `"status":"OK"`) {
		t.Errorf("Línea 2 debería contener '\"status\":\"OK\"', obtenido: %s", lines[1])
	}

	// Verificar línea 3: "status":"FAILED"
	if !strings.Contains(lines[2], `"status":"FAILED"`) {
		t.Errorf("Línea 3 debería contener '\"status\":\"FAILED\"', obtenido: %s", lines[2])
	}

	// Comprobar deserialización a través de GetEvents
	events, err := svc.GetEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetEvents falló: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("Se esperaban 3 eventos deserializados, se obtuvieron %d", len(events))
	}

	if events[0].Status != nil {
		t.Errorf("Evento 0: se esperaba status nil, se obtuvo %v", *events[0].Status)
	}
	if events[1].Status == nil || *events[1].Status != logger.StatusOK {
		t.Errorf("Evento 1: se esperaba status OK, se obtuvo %v", events[1].Status)
	}
	if events[2].Status == nil || *events[2].Status != logger.StatusFailed {
		t.Errorf("Evento 2: se esperaba status FAILED, se obtuvo %v", events[2].Status)
	}
}

func TestStageTransitions(t *testing.T) {
	tempDir := t.TempDir()
	svc := logger.NewService(tempDir)
	ctx := context.Background()
	sessionID := "test-transitions"

	stages := []struct {
		stage  logger.Stage
		action string
		status *logger.Status
	}{
		{stage: logger.StageRead, action: "Explorando repositorio", status: nil},
		{stage: logger.StagePending, action: "Modificando archivo x", status: nil},
		{stage: logger.StageFinish, action: "Tarea completada", status: logger.StatusPtr(logger.StatusOK)},
	}

	for _, s := range stages {
		evt := &logger.Event{
			SessionID: sessionID,
			AgentID:   "coder-1",
			Role:      "implementer",
			Action:    s.action,
			Stage:     s.stage,
			Status:    s.status,
		}
		if err := svc.Emit(ctx, evt); err != nil {
			t.Fatalf("Emit falló para etapa %s: %v", s.stage, err)
		}
	}

	events, err := svc.GetEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetEvents falló: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("Se esperaban 3 eventos, se obtuvieron %d", len(events))
	}

	if events[0].Stage != logger.StageRead {
		t.Errorf("Etapa 0 incorrecta: esperada READ, obtenida %s", events[0].Stage)
	}
	if events[1].Stage != logger.StagePending {
		t.Errorf("Etapa 1 incorrecta: esperada PENDING, obtenida %s", events[1].Stage)
	}
	if events[2].Stage != logger.StageFinish {
		t.Errorf("Etapa 2 incorrecta: esperada FINISH, obtenida %s", events[2].Stage)
	}
}

func TestConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	svc := logger.NewService(tempDir)
	ctx := context.Background()
	sessionID := "test-concurrent"

	const numGoroutines = 20
	const eventsPerGoroutine = 10
	totalEvents := numGoroutines * eventsPerGoroutine

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(gID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				evt := &logger.Event{
					SessionID: sessionID,
					AgentID:   fmt.Sprintf("agent-%d", gID),
					Role:      "worker",
					Action:    fmt.Sprintf("Paso %d de goroutine %d", j, gID),
					Stage:     logger.StagePending,
				}
				if err := svc.Emit(ctx, evt); err != nil {
					t.Errorf("Error emitiendo evento concurrente: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	events, err := svc.GetEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetEvents falló tras escrituras concurrentes: %v", err)
	}

	if len(events) != totalEvents {
		t.Fatalf("Se esperaban %d eventos, se encontraron %d", totalEvents, len(events))
	}

	// Comprobar que todos los eventos son válidos y tienen IDs únicos
	seenIDs := make(map[string]bool)
	for _, e := range events {
		if e.ID == "" {
			t.Errorf("Evento con ID vacío detectado")
		}
		if seenIDs[e.ID] {
			t.Errorf("ID de evento duplicado: %s", e.ID)
		}
		seenIDs[e.ID] = true
	}
}

func TestChronologicalEventReading(t *testing.T) {
	tempDir := t.TempDir()
	svc := logger.NewService(tempDir)
	ctx := context.Background()
	sessionID := "test-chrono"

	t0 := time.Now().Add(-10 * time.Minute)
	t1 := time.Now().Add(-5 * time.Minute)
	t2 := time.Now()

	e1 := &logger.Event{
		SessionID: sessionID,
		Action:    "Evento 1",
		Stage:     logger.StageRead,
		Timestamp: t0,
	}
	e2 := &logger.Event{
		SessionID: sessionID,
		Action:    "Evento 2",
		Stage:     logger.StagePending,
		Timestamp: t1,
	}
	e3 := &logger.Event{
		SessionID: sessionID,
		Action:    "Evento 3",
		Stage:     logger.StageFinish,
		Timestamp: t2,
	}

	_ = svc.Emit(ctx, e1)
	_ = svc.Emit(ctx, e2)
	_ = svc.Emit(ctx, e3)

	events, err := svc.GetEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetEvents falló: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("Se esperaban 3 eventos, se obtuvieron %d", len(events))
	}

	if events[0].Action != "Evento 1" || events[1].Action != "Evento 2" || events[2].Action != "Evento 3" {
		t.Errorf("Orden cronológico incorrecto: %+v", events)
	}
}

func TestReactiveWatch(t *testing.T) {
	tempDir := t.TempDir()
	svc := logger.NewService(tempDir)
	sessionID := "test-watch"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Iniciar Watch
	ch, err := svc.Watch(ctx, sessionID)
	if err != nil {
		t.Fatalf("Watch falló: %v", err)
	}

	receivedEvents := make([]logger.Event, 0)
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		defer close(done)
		for evt := range ch {
			mu.Lock()
			receivedEvents = append(receivedEvents, evt)
			count := len(receivedEvents)
			mu.Unlock()
			if count == 3 {
				cancel()
				return
			}
		}
	}()

	// Emitir eventos paulatinamente
	time.Sleep(50 * time.Millisecond)
	_ = svc.Emit(context.Background(), &logger.Event{
		SessionID: sessionID,
		Action:    "Watch Event 1",
		Stage:     logger.StageRead,
	})

	time.Sleep(50 * time.Millisecond)
	_ = svc.Emit(context.Background(), &logger.Event{
		SessionID: sessionID,
		Action:    "Watch Event 2",
		Stage:     logger.StagePending,
	})

	time.Sleep(50 * time.Millisecond)
	_ = svc.Emit(context.Background(), &logger.Event{
		SessionID: sessionID,
		Action:    "Watch Event 3",
		Stage:     logger.StageFinish,
		Status:    logger.StatusPtr(logger.StatusOK),
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Timeout esperando eventos reactivos de Watch")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(receivedEvents) != 3 {
		t.Fatalf("Se esperaban 3 eventos recibidos en Watch, se obtuvieron %d", len(receivedEvents))
	}

	if receivedEvents[0].Action != "Watch Event 1" ||
		receivedEvents[1].Action != "Watch Event 2" ||
		receivedEvents[2].Action != "Watch Event 3" {
		t.Errorf("Contenido inesperado de eventos en Watch: %+v", receivedEvents)
	}
}

func TestValidationErrors(t *testing.T) {
	svc := logger.NewService(t.TempDir())
	ctx := context.Background()

	// Nil event
	if err := svc.Emit(ctx, nil); err == nil {
		t.Errorf("Se esperaba error con nil event")
	}

	// Missing SessionID
	if err := svc.Emit(ctx, &logger.Event{Stage: logger.StageRead}); err == nil {
		t.Errorf("Se esperaba error por falta de SessionID")
	}

	// Invalid stage
	if err := svc.Emit(ctx, &logger.Event{SessionID: "s1", Stage: "INVALID"}); err == nil {
		t.Errorf("Se esperaba error por etapa inválida")
	}

	// Invalid status
	badStatus := logger.Status("UNKNOWN")
	if err := svc.Emit(ctx, &logger.Event{SessionID: "s1", Stage: logger.StageFinish, Status: &badStatus}); err == nil {
		t.Errorf("Se esperaba error por status inválido")
	}
}

func TestLegacyEventsReading(t *testing.T) {
	tempDir := t.TempDir()
	svc := logger.NewService(tempDir)
	ctx := context.Background()
	sessionID := "sess-legacy-events"

	legacyDir := filepath.Join(tempDir, ".harness", "sessions")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatalf("error creando directorio legacy: %v", err)
	}

	legacyFile := filepath.Join(legacyDir, sessionID+".events.jsonl")
	evtLine := `{"id":"evt-1","session_id":"` + sessionID + `","action":"Acción legacy","stage":"READ"}` + "\n"
	if err := os.WriteFile(legacyFile, []byte(evtLine), 0644); err != nil {
		t.Fatalf("error escribiendo archivo legacy: %v", err)
	}

	events, err := svc.GetEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetEvents falló en fallback legacy: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("se esperaba 1 evento de archivo legacy, obtenidos %d", len(events))
	}
	if events[0].Action != "Acción legacy" || events[0].ID != "evt-1" {
		t.Errorf("contenido inesperado de evento legacy: %+v", events[0])
	}
}
