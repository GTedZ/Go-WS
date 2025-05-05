package gows

import (
	"encoding/json"
	"slices"
	"sync"
)

// Parser and Callback types
type ParserFunc[T any] func([]byte) (bool, T)
type CallbackFunc[T any] func(T)

// messageHandler interface to unify all handler types
type messageHandler interface {
	tryParseAndCallback([]byte) bool
}

// Concrete handler for type T
type typedHandler[T any] struct {
	parser   ParserFunc[T]
	callback CallbackFunc[T]
}

func (h typedHandler[T]) tryParseAndCallback(data []byte) bool {
	if ok, val := h.parser(data); ok {
		h.callback(val)
		return true
	}
	return false
}

// messageParsers_Registry to store generic handlers
type messageParsers_Registry struct {
	mu       sync.RWMutex
	handlers []messageHandler
}

// Generic function to register parser and callback for type T
//
// if `parser` is nil, the callback will be called upon any non-error unmarshall of messages (not recommended unless it is the only message structure that is received)
func RegisterMessageParserCallback[T any](r *messageParsers_Registry, parser ParserFunc[*T], callback CallbackFunc[*T]) *typedHandler[*T] {
	r.mu.Lock()
	defer r.mu.Unlock()

	if parser == nil {
		parser = func(b []byte) (bool, *T) {
			var v T
			err := json.Unmarshal(b, &v)
			if err != nil {
				return false, nil
			}

			return true, &v
		}
	}

	messageHandler := typedHandler[*T]{parser, callback}

	r.handlers = append(r.handlers, messageHandler)

	return &messageHandler
}

// Generic function to register parser and callback for type T
//
// if `parser` is nil, the callback will be called upon any non-error unmarshall of messages (not recommended unless it is the only message structure that is received)
func DeregisterMessageParserCallback[T any](r *messageParsers_Registry, handler *typedHandler[*T]) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, h := range r.handlers {
		if typed, ok := h.(typedHandler[*T]); ok {
			if &typed == handler {
				// Remove element from slice
				r.handlers = slices.Delete(r.handlers, i, i+1)
				break
			}
		}
	}
}
