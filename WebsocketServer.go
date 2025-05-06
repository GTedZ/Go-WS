package gows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//////////////////
//////////////////
//////////////////

type WebsocketServer struct {
	upgrader               *websocket.Upgrader
	requestId_propertyName string

	nextClientId int64 // Next client ID to assign
	Clients      struct {
		Mu  sync.Mutex
		Map map[int64]*WebsocketConnection
	}

	OnUpgradeError EventEmitter[UpgradeError]
	OnConnect      EventEmitter[*WebsocketConnection]

	OnError EventEmitter[Error]
	OnClose EventEmitter[CloseEvent]

	OnAnyMessage EventEmitter[IncomingMessage]

	// This is an eventEmitter for incoming requests from the client socket
	OnRequest EventEmitter[ServerIncomingRequest]
	// This is an eventEmitter for incoming response from the client socket of a request sent from the server
	OnRequestResponse EventEmitter[IncomingMessage]

	// In order to register the parsed messages
	MessageParserRegistry messageParsers_Registry
	OnParsedMessage       EventEmitter[IncomingMessage]

	OnMessage EventEmitter[IncomingMessage]
}

func (ws_server *WebsocketServer) SetCheckOriginFunc(checkOriginFunc func(r *http.Request) bool) {
	var upgraderFunc func(r *http.Request) bool

	if checkOriginFunc != nil {
		upgraderFunc = checkOriginFunc
	} else {
		upgraderFunc = func(r *http.Request) bool {
			return true // Allow all origins
		}
	}

	ws_server.upgrader = &websocket.Upgrader{
		CheckOrigin: upgraderFunc,
	}
}

func (ws_server *WebsocketServer) SetRequestIdProperty(propertyName string) error {
	if propertyName == "" {
		return fmt.Errorf("property must be of non-zero value, an empty string is not valid")
	}
	ws_server.requestId_propertyName = propertyName
	return nil
}

func (ws_server *WebsocketServer) GetRequestIdProperty() string {
	return ws_server.requestId_propertyName
}

// Create a function to create a new websocket server, with a given address and port
func NewWebSocketServer(checkOriginFunc func(r *http.Request) bool) *WebsocketServer {
	websocketServer := &WebsocketServer{
		requestId_propertyName: "Id",
	}
	websocketServer.SetCheckOriginFunc(checkOriginFunc)
	websocketServer.Clients.Map = make(map[int64]*WebsocketConnection)

	return websocketServer
}

func (ws_server *WebsocketServer) onConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := ws_server.upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws_server.OnUpgradeError.emit(UpgradeError{Err: err, Request: r})
		return
	}
	defer conn.Close()

	ws_connection := &WebsocketConnection{
		id:          ws_server.nextClientId,
		Id:          ws_server.nextClientId,
		parent:      ws_server,
		Conn:        conn,
		OrigRequest: r,
	}
	ws_connection.requestResponseAwaiters.Map = make(map[int64]requestResponseAwaiter)
	ws_connection.CustomData.Map = make(map[string]string)
	ws_server.nextClientId++

	// Adding the client
	ws_server.Clients.Mu.Lock()
	ws_server.Clients.Map[ws_connection.id] = ws_connection
	ws_server.Clients.Mu.Unlock()

	// Emitting new connection
	ws_server.OnConnect.emit(ws_connection)

	conn.SetCloseHandler(
		func(code int, text string) error {
			ws_server.Clients.Mu.Lock()
			delete(ws_server.Clients.Map, ws_connection.id)
			ws_server.Clients.Mu.Unlock()

			ws_server.OnClose.emit(CloseEvent{Connection: ws_connection, Code: code, Text: text})
			ws_connection.OnClose.emit(SocketCloseEvent{Code: code, Text: text})
			return nil
		},
	)

	// Handle the connection (e.g., read/write messages)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			ws_server.OnError.emit(Error{Connection: ws_connection, Err: err})
			ws_connection.OnError.emit(SocketError{Err: err})

			ws_server.Clients.Mu.Lock()
			_, exists := ws_server.Clients.Map[ws_connection.id]
			ws_server.Clients.Mu.Unlock()
			if !exists {
				break
			}

			continue
		}

		ws_server.OnAnyMessage.emit(IncomingMessage{Connection: ws_connection, Message: msg})
		ws_connection.OnAnyMessage.emit(SocketMessage{Message: msg})

		ws_connection.handleMessage(msg)
	}
}

// 'Listen' starts the WebSocket server and listens for incoming connections on the specified paths.
//
// This function is blocking and will not return until the server is stopped.
//
// If no paths are provided, it will listen on the root path ("/").
func (ws_server *WebsocketServer) Listen(port int, paths ...string) error {
	for _, path := range paths {
		http.HandleFunc(path, ws_server.onConnect)
	}
	if len(paths) == 0 {
		http.HandleFunc("/", ws_server.onConnect)
	}

	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	return err
}

///////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////

type WebsocketConnection struct {
	id int64
	Id int64 // Read-only unique ID for the connection (set automatically by the server)

	parent *WebsocketServer

	writeMu     sync.Mutex
	Conn        *websocket.Conn
	OrigRequest *http.Request

	OnError EventEmitter[SocketError]
	OnClose EventEmitter[SocketCloseEvent]

	OnAnyMessage EventEmitter[SocketMessage]

	// This is an eventEmitter for incoming requests from the client socket
	OnRequest EventEmitter[IncomingRequest]

	// This is an eventEmitter for incoming response from the client socket of a request sent from the server
	OnRequestResponse EventEmitter[SocketMessage]

	OnParsedMessage EventEmitter[SocketMessage]
	OnMessage       EventEmitter[SocketMessage]

	requestResponseAwaiters struct {
		Mu  sync.Mutex
		Map map[int64]requestResponseAwaiter
	}

	// # Not used by library
	//
	// This is simply left to the developer to use as they please
	//
	// In the case of complex AllowOrigin configurations, this can help store metadata to match a connection to a user or any application-logic data
	CustomData struct {
		Mu  sync.Mutex
		Map map[string]string
	}
}

func (ws_connection *WebsocketConnection) Close() error {
	return ws_connection.Conn.Close()
}

func (ws_connection *WebsocketConnection) SendText(msg string) error {
	ws_connection.writeMu.Lock()
	defer ws_connection.writeMu.Unlock()
	return ws_connection.Conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

func (ws_connection *WebsocketConnection) SendJSON(msg interface{}) error {
	ws_connection.writeMu.Lock()
	defer ws_connection.writeMu.Unlock()
	return ws_connection.Conn.WriteJSON(msg)
}

func (ws_connection *WebsocketConnection) handleMessage(msg []byte) {

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
		received_requestId_interface, isPresent := parsedData[ws_connection.parent.requestId_propertyName]
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
			ws_connection.requestResponseAwaiters.Mu.Lock()
			responseAwaiter, exists := ws_connection.requestResponseAwaiters.Map[received_requestId]
			ws_connection.requestResponseAwaiters.Mu.Unlock()
			fmt.Printf("[SERVER] ResponseAwaiter: %v, exists: %v\n", responseAwaiter, exists)
			if exists {
				responseAwaiter.channel <- msg
				isRequestResponse = true
			} else {
				ws_connection.parent.OnRequest.emit(ServerIncomingRequest{requestId: received_requestId, propertyName: ws_connection.parent.requestId_propertyName, WebsocketConnection: ws_connection, Data: msg})
				ws_connection.OnRequest.emit(IncomingRequest{requestId: received_requestId, propertyName: ws_connection.parent.requestId_propertyName, conn: ws_connection.Conn, Data: msg})
				isRequest = true
			}
		}
	}
	if isRequestResponse {
		ws_connection.parent.OnRequestResponse.emit(IncomingMessage{Connection: ws_connection, Message: msg})
		ws_connection.OnRequestResponse.emit(SocketMessage{Message: msg})
		return
	}
	if isRequest {
		return
	}

	// Checking for parsed messages
	isParsedMessage := false
	for _, parser := range ws_connection.parent.MessageParserRegistry.handlers {
		isParsedMessage = parser.tryParseAndCallback(msg)
		if isParsedMessage {
			break
		}
	}

	if isParsedMessage {
		ws_connection.parent.OnParsedMessage.emit(IncomingMessage{Connection: ws_connection, Message: msg})
		ws_connection.OnParsedMessage.emit(SocketMessage{Message: msg})
		return
	}

	ws_connection.parent.OnMessage.emit(IncomingMessage{Connection: ws_connection, Message: msg})
	ws_connection.OnMessage.emit(SocketMessage{Message: msg})
}

func (ws_connection *WebsocketConnection) SendPreparedMessage(pm *websocket.PreparedMessage) error {
	return ws_connection.Conn.WritePreparedMessage(pm)
}

// SendRequest sends msg to the server and waits for a response or timeout (if set).
//
// The response is unmarshalled into the provided variable v.
//
// If the message failed to send, or the response is not received within the timeout, an error is returned along with the respective hasTimedOut value.
func (ws_connection *WebsocketConnection) SendRequest(request any, response any, opt_params *SendRequestParams) (hasTimedOut bool, err error) {
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
		ws_connection.requestResponseAwaiters.Mu.Lock()
		delete(ws_connection.requestResponseAwaiters.Map, newResponseAwaiter.expectedRequestId)
		ws_connection.requestResponseAwaiters.Mu.Unlock()
	}()

	ws_connection.requestResponseAwaiters.Mu.Lock()
	ws_connection.requestResponseAwaiters.Map[newResponseAwaiter.expectedRequestId] = newResponseAwaiter
	ws_connection.requestResponseAwaiters.Mu.Unlock()

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
	originalMap[ws_connection.parent.requestId_propertyName] = newResponseAwaiter.expectedRequestId

	// TODO: remove this snippet
	msg, _ := json.Marshal(originalMap)
	fmt.Println("[SERVER] Sending request:", string(msg))
	// TODO: remove this snippet

	// Send the final map
	ws_connection.writeMu.Lock()
	err = ws_connection.Conn.WriteJSON(originalMap)
	ws_connection.writeMu.Unlock()
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
