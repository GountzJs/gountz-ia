package events

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Disposable represents a resource or subscription that can be cancelled or cleaned up.
type Disposable interface {
	Dispose()
}

// DisposableFunc is an adapter to allow the use of ordinary functions as Disposable.
type DisposableFunc func()

// Dispose calls f().
func (f DisposableFunc) Dispose() {
	if f != nil {
		f()
	}
}

// EventHandler is the callback invoked when a Pub/Sub event is received.
type EventHandler func(ctx context.Context, payload any) error

// RequestHandler is the callback invoked to process an RPC request and return a response.
type RequestHandler func(ctx context.Context, payload any) (any, error)

// EventBus provides decoupled asynchronous Pub/Sub messaging and synchronous RPC communication.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]map[uint64]EventHandler
	nextSubID   uint64
	rpcHandlers map[string]RequestHandler
}

// NewEventBus creates and initializes a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string]map[uint64]EventHandler),
		rpcHandlers: make(map[string]RequestHandler),
	}
}

// Publish emits an event to all registered subscribers.
func (b *EventBus) Publish(ctx context.Context, event string, payload any) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	b.mu.RLock()
	handlersMap, exists := b.subscribers[event]
	if !exists || len(handlersMap) == 0 {
		b.mu.RUnlock()
		return nil
	}

	handlers := make([]EventHandler, 0, len(handlersMap))
	for _, h := range handlersMap {
		handlers = append(handlers, h)
	}
	b.mu.RUnlock()

	var errs []error
	for _, h := range handlers {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if err := h(ctx, payload); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// Subscribe registers an event listener for a given event topic.
func (b *EventBus) Subscribe(event string, handler EventHandler) Disposable {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextSubID++
	id := b.nextSubID

	if _, exists := b.subscribers[event]; !exists {
		b.subscribers[event] = make(map[uint64]EventHandler)
	}
	b.subscribers[event][id] = handler

	return DisposableFunc(func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if subs, exists := b.subscribers[event]; exists {
			delete(subs, id)
			if len(subs) == 0 {
				delete(b.subscribers, event)
			}
		}
	})
}

// Respond registers a request-response RPC handler for a given topic.
// Returns an error if a handler is already registered for this topic.
func (b *EventBus) Respond(topic string, handler RequestHandler) (Disposable, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.rpcHandlers[topic]; exists {
		return nil, fmt.Errorf("[EventBus] RPC handler for topic %q is already registered", topic)
	}

	b.rpcHandlers[topic] = handler

	return DisposableFunc(func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.rpcHandlers, topic)
	}), nil
}

// Request sends an RPC request to the handler registered on the topic and awaits the response.
func (b *EventBus) Request(ctx context.Context, topic string, payload any) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	b.mu.RLock()
	handler, exists := b.rpcHandlers[topic]
	b.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("[EventBus] no RPC handler registered for topic %q", topic)
	}

	return handler(ctx, payload)
}

// Clear removes all event subscribers and RPC handlers.
func (b *EventBus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscribers = make(map[string]map[uint64]EventHandler)
	b.rpcHandlers = make(map[string]RequestHandler)
}
