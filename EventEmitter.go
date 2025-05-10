package gows

import "fmt"

// For events with no return value
type VoidHandler[T any] func(T)

type EventEmitter[T any] struct {
	listeners []VoidHandler[T]
}

func (e *EventEmitter[T]) Subscribe(handler VoidHandler[T]) {
	e.listeners = append(e.listeners, handler)
}

func (e *EventEmitter[T]) Unsubscribe(handler VoidHandler[T]) {
	for i, h := range e.listeners {
		if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
			// Remove handler
			e.listeners = append(e.listeners[:i], e.listeners[i+1:]...)
			break
		}
	}
}

func (e *EventEmitter[T]) emit(arg T) {
	for _, handler := range e.listeners {
		go handler(arg)
	}
}

// Not sure if this needs to be public
func (e *EventEmitter[T]) Emit(arg T) {
	for _, handler := range e.listeners {
		go handler(arg)
	}
}

func (e *EventEmitter[T]) clear(arg T) {
	for _, listener := range e.listeners {
		e.Unsubscribe(listener)
	}
}

// // For events where handlers return a value
// type ReturnHandler[T any, R any] func(T) R

// type EventEmitterWithReturn[T any, R any] struct {
// 	listeners []ReturnHandler[T, R]
// }

// func (e *EventEmitterWithReturn[T, R]) Subscribe(handler ReturnHandler[T, R]) {
// 	e.listeners = append(e.listeners, handler)
// }

// func (e *EventEmitterWithReturn[T, R]) Unsubscribe(handler ReturnHandler[T, R]) {
// 	for i, h := range e.listeners {
// 		if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
// 			// Remove handler
// 			e.listeners = append(e.listeners[:i], e.listeners[i+1:]...)
// 			break
// 		}
// 	}
// }

// func (e *EventEmitterWithReturn[T, R]) emit(arg T) []R {
// 	var results []R
// 	for _, handler := range e.listeners {
// 		results = append(results, handler(arg))
// 	}
// 	return results
// }

// // Not sure if this needs to be public
// func (e *EventEmitterWithReturn[T, R]) Emit(arg T) []R {
// 	var results []R
// 	for _, handler := range e.listeners {
// 		results = append(results, handler(arg))
// 	}
// 	return results
// }
