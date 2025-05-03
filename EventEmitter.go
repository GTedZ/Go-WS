package gows

// For events with no return value
type VoidHandler[T any] func(T)

type EventEmitter[T any] struct {
	listeners []VoidHandler[T]
}

func (e *EventEmitter[T]) Subscribe(handler VoidHandler[T]) {
	e.listeners = append(e.listeners, handler)
}

func (e *EventEmitter[T]) emit(arg T) {
	for _, handler := range e.listeners {
		handler(arg)
	}
}

// For events where handlers return a value
type ReturnHandler[T any, R any] func(T) R

type EventEmitterWithReturn[T any, R any] struct {
	listeners []ReturnHandler[T, R]
}

func (e *EventEmitterWithReturn[T, R]) Subscribe(handler ReturnHandler[T, R]) {
	e.listeners = append(e.listeners, handler)
}

func (e *EventEmitterWithReturn[T, R]) emit(arg T) []R {
	var results []R
	for _, handler := range e.listeners {
		results = append(results, handler(arg))
	}
	return results
}
