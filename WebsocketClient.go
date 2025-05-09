package gows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type WebsocketClient struct {
	URL                    string
	requestId_propertyName string

	Conn   *websocket.Conn
	closed bool

	writeMu sync.Mutex

	OnError EventEmitter[SocketError]
	OnClose EventEmitter[SocketCloseEvent]

	OnAnyMessage EventEmitter[SocketMessage]

	// This is an eventEmitter for incoming requests from the server socket
	OnRequest EventEmitter[IncomingRequest]

	// This is an eventEmitter for incoming responses from the server to a request sent from the client
	OnRequestResponse EventEmitter[SocketMessage]

	// In order to register the parsed messages
	MessageParserRegistry messageParsers_Registry
	OnParsedMessage       EventEmitter[SocketMessage]
	OnMessage             EventEmitter[SocketMessage]

	requestResponseAwaiters struct {
		Mu  sync.Mutex
		Map map[int64]requestResponseAwaiter
	}
}

func (ws_client *WebsocketClient) SetRequestIdProperty(propertyName string) error {
	if propertyName == "" {
		return fmt.Errorf("property must be of non-zero value, an empty string is not valid")
	}
	ws_client.requestId_propertyName = propertyName
	return nil
}

func (ws_client *WebsocketClient) GetRequestIdProperty() string {
	return ws_client.requestId_propertyName
}

func NewWebsocketClient(url string) *WebsocketClient {
	ws_client := &WebsocketClient{
		URL:                    url,
		requestId_propertyName: "Id",
	}
	ws_client.requestResponseAwaiters.Map = make(map[int64]requestResponseAwaiter)
	return ws_client
}

///////////////////////////
///////////////////////////
///////////////////////////

func (ws_client *WebsocketClient) Connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(ws_client.URL, nil)
	if err != nil {
		return err
	}
	ws_client.Conn = conn

	ws_client.Conn.SetCloseHandler(ws_client.onClose)

	ws_client.setupGracefulShutdown()

	go ws_client.readMessages()

	return nil
}

func (ws_client *WebsocketClient) onClose(code int, text string) error {
	ws_client.closed = true
	ws_client.OnClose.emit(SocketCloseEvent{Code: code, Text: text})
	return nil
}

func (ws_client *WebsocketClient) setupGracefulShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received")

		ws_client.Close()
		os.Exit(0)
	}()
}

func (ws_client *WebsocketClient) readMessages() {
	for {
		_, msg, err := ws_client.Conn.ReadMessage()
		if err != nil {
			if ws_client.closed {
				break
			}
			ws_client.OnError.emit(SocketError{Err: err})
			continue
		}

		ws_client.OnAnyMessage.emit(SocketMessage{Message: msg})

		ws_client.handleMessage(msg)
	}
}

func (ws_client *WebsocketClient) handleMessage(msg []byte) {

	// Checking for private messages
	//
	// If found, send the []byte to the channel waiting for it
	decoderInt64 := json.NewDecoder(bytes.NewReader(msg))
	decoderInt64.UseNumber()
	isRequestResponse := false
	isRequest := false
	var parsedData map[string]interface{}
	err := decoderInt64.Decode(&parsedData)
	if err == nil {
		received_requestId_interface, isPresent := parsedData[ws_client.requestId_propertyName]
		var received_requestId int64
		ok := false
		if isPresent {
			switch v := received_requestId_interface.(type) {
			case json.Number:
				received_requestId, err = v.Int64()
				if err == nil {
					ok = true
				}
			case float64:
				received_requestId = int64(v)
				ok = true
			}
		}
		if ok {
			ws_client.requestResponseAwaiters.Mu.Lock()
			responseAwaiter, exists := ws_client.requestResponseAwaiters.Map[received_requestId]
			ws_client.requestResponseAwaiters.Mu.Unlock()
			if exists {
				responseAwaiter.channel <- msg
				isRequestResponse = true
			} else {
				ws_client.OnRequest.emit(IncomingRequest{requestId: received_requestId, propertyName: ws_client.requestId_propertyName, conn: ws_client.Conn, Data: msg})
				isRequest = true
			}
		}
	}
	if isRequestResponse {
		ws_client.OnRequestResponse.emit(SocketMessage{Message: msg})
		return
	}
	if isRequest {
		return
	}

	// Checking for parsed messages
	isParsedMessage := false
	for _, parser := range ws_client.MessageParserRegistry.handlers {
		isParsedMessage = parser.tryParseAndCallback(msg)
		if isParsedMessage {
			break
		}
	}

	if isParsedMessage {
		ws_client.OnParsedMessage.emit(SocketMessage{Message: msg})
		return
	}

	ws_client.OnMessage.emit(SocketMessage{Message: msg})
}

func (ws_client *WebsocketClient) IsClosed() bool {
	return ws_client.closed
}

func (ws_client *WebsocketClient) Close() error {
	return ws_client.onClose(websocket.CloseNormalClosure, "Manual shutdown")
}

func (ws_client *WebsocketClient) SendText(msg string) error {
	ws_client.writeMu.Lock()
	defer ws_client.writeMu.Unlock()
	return ws_client.Conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

func (ws_client *WebsocketClient) SendJSON(msg interface{}) error {
	ws_client.writeMu.Lock()
	defer ws_client.writeMu.Unlock()
	return ws_client.Conn.WriteJSON(msg)
}

// SendRequest sends msg to the server and waits for a response or timeout (if set).
//
// The response is unmarshalled into the provided variable v.
//
// If the message failed to send, or the response is not received within the timeout, an error is returned along with the respective hasTimedOut value.
func (ws_client *WebsocketClient) SendRequest(request any, response any, opt_params *SendRequestParams) (hasTimedOut bool, err error) {
	var params SendRequestParams
	if opt_params == nil {
		params.Timeout = 4 * time.Second
	} else {
		params = *opt_params
	}

	requestId := generateSecureRandomInt64()
	newResponseAwaiter := requestResponseAwaiter{
		channel:           make(chan []byte),
		expectedRequestId: requestId,
	}
	defer func() {
		ws_client.requestResponseAwaiters.Mu.Lock()
		delete(ws_client.requestResponseAwaiters.Map, newResponseAwaiter.expectedRequestId)
		ws_client.requestResponseAwaiters.Mu.Unlock()
	}()

	ws_client.requestResponseAwaiters.Mu.Lock()
	ws_client.requestResponseAwaiters.Map[newResponseAwaiter.expectedRequestId] = newResponseAwaiter
	ws_client.requestResponseAwaiters.Mu.Unlock()

	/////////////////////////////////////////
	// Serialize user's msg into JSON
	originalJson, err := json.Marshal(request)
	if err != nil {
		return false, err
	}

	// Deserialize into a map
	var originalMap map[string]interface{}
	err = json.Unmarshal(originalJson, &originalMap)
	if err != nil {
		return false, err
	}

	// Inject your internal fields
	originalMap[ws_client.requestId_propertyName] = newResponseAwaiter.expectedRequestId

	// Send the final map
	ws_client.writeMu.Lock()
	err = ws_client.Conn.WriteJSON(originalMap)
	ws_client.writeMu.Unlock()
	if err != nil {
		return false, err
	}
	/////////////////////////////////////////

	var timeoutChan <-chan time.Time
	if params.Timeout > 0 {
		timeoutChan = time.After(params.Timeout)
	}

	select {
	case responseData := <-newResponseAwaiter.channel:
		err := json.Unmarshal(responseData, response)
		return false, err
	case <-timeoutChan:
		// if timeout is nil, this case is never selected
		return true, fmt.Errorf("timeout")
	}
}
