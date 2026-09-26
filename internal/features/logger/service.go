package logger

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Service define el contrato de casos de uso para emitir, consultar y observar eventos de sesión.
type Service interface {
	Emit(ctx context.Context, evt *Event) error
	GetEvents(ctx context.Context, sessionID string) ([]Event, error)
	Watch(ctx context.Context, sessionID string) (<-chan Event, error)
}

type loggerService struct {
	baseDir string
	store   Store
	mu      sync.RWMutex
	subs    map[string][]chan Event
}

// Option permite configurar opciones para el servicio de logger.
type Option func(*loggerService)

// WithBaseDir configura el directorio base para el logger.
func WithBaseDir(baseDir string) Option {
	return func(s *loggerService) {
		s.baseDir = baseDir
	}
}

// WithStore inyecta un almacén de eventos personalizado.
func WithStore(store Store) Option {
	return func(s *loggerService) {
		s.store = store
	}
}

func resolveEventsDir(dir string) string {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	clean := filepath.Clean(dir)
	if filepath.Base(clean) == "sessions" && filepath.Base(filepath.Dir(clean)) == ".harness" {
		return clean
	}
	return filepath.Join(clean, ".harness", "sessions")
}

// NewService crea una nueva instancia de Service con baseDir y opciones configuradas.
func NewService(baseDir string, opts ...Option) Service {
	svc := &loggerService{
		baseDir: baseDir,
		subs:    make(map[string][]chan Event),
	}

	for _, opt := range opts {
		opt(svc)
	}

	if svc.store == nil {
		eventsDir := resolveEventsDir(svc.baseDir)
		svc.store = NewFileStore(eventsDir)
	}

	return svc
}

// Emit valida y persiste un evento, notificando a observadores activos.
func (s *loggerService) Emit(ctx context.Context, evt *Event) error {
	if evt == nil {
		return errors.New("evento no puede ser nil")
	}

	if err := evt.Validate(); err != nil {
		return err
	}

	if evt.ID == "" {
		evt.ID = GenerateEventID()
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}

	if err := s.store.Append(evt.SessionID, evt); err != nil {
		return err
	}

	s.mu.RLock()
	if chans, ok := s.subs[evt.SessionID]; ok {
		for _, ch := range chans {
			select {
			case ch <- *evt:
			default:
			}
		}
	}
	s.mu.RUnlock()

	return nil
}

// GetEvents obtiene todos los eventos registrados para la sesión en orden cronológico.
func (s *loggerService) GetEvents(ctx context.Context, sessionID string) ([]Event, error) {
	if sessionID == "" {
		return nil, errors.New("sessionID no puede estar vacío")
	}
	return s.store.ReadEvents(sessionID)
}

// Watch retorna un canal que emite eventos existentes y continúa transmitiendo nuevos eventos en tiempo real.
func (s *loggerService) Watch(ctx context.Context, sessionID string) (<-chan Event, error) {
	if sessionID == "" {
		return nil, errors.New("sessionID no puede estar vacío")
	}

	out := make(chan Event, 100)
	inProc := make(chan Event, 50)

	s.mu.Lock()
	s.subs[sessionID] = append(s.subs[sessionID], inProc)
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			chans := s.subs[sessionID]
			for i, ch := range chans {
				if ch == inProc {
					s.subs[sessionID] = append(chans[:i], chans[i+1:]...)
					break
				}
			}
			if len(s.subs[sessionID]) == 0 {
				delete(s.subs, sessionID)
			}
			s.mu.Unlock()
			close(out)
		}()

		seenIDs := make(map[string]bool)

		syncFromFile := func() {
			events, err := s.store.ReadEvents(sessionID)
			if err != nil {
				return
			}
			for _, evt := range events {
				if !seenIDs[evt.ID] {
					seenIDs[evt.ID] = true
					select {
					case out <- evt:
					case <-ctx.Done():
						return
					}
				}
			}
		}

		syncFromFile()

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case evt := <-inProc:
				if !seenIDs[evt.ID] {
					seenIDs[evt.ID] = true
					select {
					case out <- evt:
					case <-ctx.Done():
						return
					}
				}
			case <-ticker.C:
				syncFromFile()
			}
		}
	}()

	return out, nil
}
