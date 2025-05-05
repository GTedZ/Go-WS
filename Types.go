package gows

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
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

type IncomingMessage struct {
	Connection *WebsocketConnection
	Message    []byte
}

type ServerIncomingRequest struct {
	requestId    int64
	propertyName string

	WebsocketConnection *WebsocketConnection
	Data                []byte
}

func (request *ServerIncomingRequest) Reply(data interface{}) error {
	// Serialize user's msg into JSON
	originalJson, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Deserialize into a map
	var originalMap map[string]interface{}
	err = json.Unmarshal(originalJson, &originalMap)
	if err != nil {
		return err
	}

	// Inject your internal fields
	originalMap[request.propertyName] = request.requestId

	return request.WebsocketConnection.SendJSON(originalMap)
}

type IncomingRequest struct {
	requestId    int64
	propertyName string
	conn         *websocket.Conn

	Data []byte
}

func (request *IncomingRequest) Reply(data interface{}) error {
	// Serialize user's msg into JSON
	originalJson, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Deserialize into a map
	var originalMap map[string]interface{}
	err = json.Unmarshal(originalJson, &originalMap)
	if err != nil {
		return err
	}

	// Inject your internal fields
	originalMap[request.propertyName] = request.requestId

	return request.conn.WriteJSON(originalMap)
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
