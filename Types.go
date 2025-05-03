package gows

import (
	"net/http"
	"time"
)

type UpgradeError struct {
	Err     error
	Request *http.Request
}

type Error struct {
	Connection *WebsocketConnection
	Err        error
}

type CloseEvent struct {
	Connection *WebsocketConnection
	Code       int
	Text       string
}

type Message struct {
	Connection *WebsocketConnection
	Message    []byte
}

///

type SocketError struct {
	Err error
}

type SocketCloseEvent struct {
	Code int
	Text string
}

type SocketMessage struct {
	Message []byte
}

type SendRequestParams struct {
	Timeout time.Duration
}

type requestResponseAwaiter struct {
	channel           chan []byte
	expectedRequestId int64
}
