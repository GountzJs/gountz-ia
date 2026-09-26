package logger

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Store define el contrato para persistir y consultar eventos de sesión.
type Store interface {
	Append(sessionID string, evt *Event) error
	ReadEvents(sessionID string) ([]Event, error)
	EventFilePath(sessionID string) string
}

// FileStore implementa Store persistiendo eventos en formato JSON Lines (.events.jsonl).
type FileStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileStore crea una nueva instancia de FileStore en la ruta provista.
func NewFileStore(baseDir string) *FileStore {
	return &FileStore{
		baseDir: baseDir,
	}
}

// EventFilePath retorna la ruta completa del archivo .events.jsonl para una sesión dada.
func (f *FileStore) EventFilePath(sessionID string) string {
	return filepath.Join(f.baseDir, sessionID+".events.jsonl")
}

// Append persiste de forma segura y concurrente un evento al final del archivo de eventos de la sesión.
func (f *FileStore) Append(sessionID string, evt *Event) error {
	if sessionID == "" {
		return errors.New("sessionID no puede estar vacío")
	}
	if evt == nil {
		return errors.New("evento no puede ser nil")
	}

	if evt.SessionID == "" {
		evt.SessionID = sessionID
	}
	if evt.ID == "" {
		evt.ID = GenerateEventID()
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}

	if err := evt.Validate(); err != nil {
		return err
	}

	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("error al serializar evento: %w", err)
	}
	data = append(data, '\n')

	f.mu.Lock()
	defer f.mu.Unlock()

	if err := os.MkdirAll(f.baseDir, 0755); err != nil {
		return fmt.Errorf("error al crear directorio base de eventos: %w", err)
	}

	filePath := f.EventFilePath(sessionID)
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error al abrir archivo de eventos: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("error al escribir evento en disco: %w", err)
	}

	return nil
}

// ReadEvents lee y deserializa en orden cronológico todos los eventos registrados para una sesión.
func (f *FileStore) ReadEvents(sessionID string) ([]Event, error) {
	if sessionID == "" {
		return nil, errors.New("sessionID no puede estar vacío")
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	filePath := f.EventFilePath(sessionID)
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil
		}
		return nil, fmt.Errorf("error al abrir archivo de eventos: %w", err)
	}
	defer file.Close()

	var events []Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var evt Event
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			return nil, fmt.Errorf("error al deserializar línea de evento: %w", err)
		}
		events = append(events, evt)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error al escanear archivo de eventos: %w", err)
	}

	return events, nil
}
