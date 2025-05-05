# Go-WS: A High-Level WebSocket Library for Go

Go-WS is a robust and ergonomic wrapper around Gorilla WebSocket, designed to simplify and accelerate WebSocket communication for both server and client implementations in Go. It introduces high-level event emitters, request-response abstractions, and message parsing logic, making it ideal for building scalable real-time applications.

This README guides you through the usage of the library using the `server.go` example, which demonstrates all major features of Go-WS.

---

## Features

* Easy WebSocket server and client setup
* Event-driven architecture (connection, disconnection, errors, messages)
* Request-response pattern built-in
* Custom message parsing and typed handler registration
* Graceful connection handling with ID assignment
* Thread-safe message sending

---

## Installation

```bash
go get github.com/GTedZ/Go-WS
```

---

## Getting Started

### 1. Setting Up a WebSocket Server

```go
server := gows.NewWebSocketServer(nil)
```

Use `server.Listen(port)` to start listening.

```go
go server.Listen(3000)
```

### 2. Configuring Connection Origin Check (can be left as `nil`) [OPTIONAL]

This allows the server to set rules on who to accept based on the return value of the function provided

#### Use Cases:
- Set rules on who to accept and who to reject via the `*http.Request` variable
- Can be used to set complex rules based on application-logic, metadata checks

```go
server.SetCheckOriginFunc(func(r *http.Request) bool {
    return true // Allow all origins
})
```
OR
```go
server.SetCheckOriginFunc(nil)  // nil means accept-all (similar to the above example)
```

### 3. Handling Events

```go
server.OnConnect.Subscribe(func(conn *gows.WebsocketConnection) {
    // Handle new connection
})

server.OnRequest.Subscribe(func(req gows.ServerIncomingRequest) {
    // Handle incoming structured requests
})
```

### 4. Adding connection-level metadata:

You can use `OnConnect`'s connection to check the `*http.Request` that was used to connect, alongside the `CustomData` property of the connection to set any metadata you would like into the structure.

### Example:

```go
server.SetCheckOriginFunc(func(r *http.Request) bool {
    username := r.Header.Get("username")
    if username == "" {
        return false
    }

    return true;
})

server.OnConnect.Subscribe(func(conn *gows.WebsocketConnection) {
		wc.CustomData.Mu.Lock()
		wc.CustomData.Mu.Unlock()
		wc.CustomData.Map["username"] = conn.OrigRequest.Header.Get("username")
})
```

### 5. Sending Text and JSON Messages

```go
conn.SendText("Hello")
conn.SendJSON(struct { Message string }{Message: "Hello"})
```

---

## WebSocket Client

### 1. Creating and Connecting

```go
client := gows.NewWebsocketClient("ws://localhost:3000")
err := client.Connect()
```

### 2. Receiving Messages

```go
client.OnMessage.Subscribe(func(msg gows.SocketMessage) {
    fmt.Println(string(msg.Message))
})
```

---

## Request-Response Pattern

The library implements a Request/Response pattern that can be used to quickly and synchronously send and receive messages.

It uses a discrete `requestId_propertyName` (by default `"Id"`) silently embedded alongside your request struct, that will be used to automatically match with the other party's `Reply` message

### NOTE:
- The request/response schema is bidirectional, both the server and client can send, receive and handle both requests and responses
- Both parties MUST have the same `requestId_propertyName` registered, you can register your own custom propertyName via `server.SetRequestIdProperty()` or `client.SetRequestIdProperty`

### Client Sending Request

```go
var response MyResponse
client.SendRequest(MyStruct{...}, &response, nil)
```

### Server Handling Request

```go
server.OnRequest.Subscribe(func(req gows.ServerIncomingRequest) {
    var parsedData MyRequest
    json.Unmarshal(req.Data, &parsedData)
    req.Reply(MyResponse{...})
})
```

---

## Message Parsers

Message parsers setup the retrieval of expected JSON structures, with the ability to provide a custom Parser and a handler to receive it

```go
gows.RegisterMessageParserCallback(
    &server.MessageParserRegistry,
    <nil> OR func(b []byte) (bool, *MyStruct),
    func(s *YourType),
)
```

### Server-Side Parser Registration:
Register a typed parser for incoming JSON structures:

```go
gows.RegisterMessageParserCallback(
    &server.MessageParserRegistry,
    func(b []byte) (bool, *MyStruct) {
        var s MyStruct
        err := json.Unmarshal(b, &s)
        return err == nil && s.Field != "", &s
    },
    func(s *MyStruct) {
        fmt.Println("Parsed struct:", s)
    },
)
```

### Client-Side Parser Registration:
Register a typed parser for incoming JSON structures:

```go
gows.RegisterMessageParserCallback(
    &client.MessageParserRegistry,
    func(b []byte) (bool, *MyStruct) {
        var s MyStruct
        err := json.Unmarshal(b, &s)
        return err == nil && s.Field != "", &s
    },
    func(s *MyStruct) {
        fmt.Println("Parsed struct:", s)
    },
)
```

### Parser Function: `func(b []byte) (isValid bool, result *YourType)` OR `nil`

The parser function (passed as the second argument in the `gows.RegisterMessageParserCallback` function) is the function called on every message in order to check if it is a valid message, here's a brief explanation of the variables:
- `b []byte`: This is the raw message received in the websocket stream
- `isValid bool`: When the parser function returns `true`, the message is accepted as a parsed message, and forwards the returned `result` value to the callback function
- `result *YourType`: Once the message is accepted, this value will be forwarded as-is to the registered callback function.

#### IMPORTANT: Message parsers aren't limited to structs only, but when providing a `nil` parser, it is expected to be a struct, and any non-error JSON unmarshall of the struct will be accepted as a valid parsed message.

---

## Events Summary

* `OnConnect`, `OnClose`, `OnError`, `OnAnyMessage`, `OnMessage`, `OnParsedMessage`
* `OnRequest` and `OnRequestResponse`
* All events are `EventEmitter[T]` allowing `Subscribe()` and `Unsubscribe()`

---

## Contributions

Feel free to submit issues and pull requests. Feedback and improvements are welcome.

## Support

If you would like to support my projects, you can donate to the following addresses:

BTC: `19XC9b9gkbNVTB9eTPzjByijmx8gNd2qVR`

ETH (ERC20): `0x7e5db6e135da1f4d5de26f7f7d4ed10dada20478`

USDT (TRC20): `TLHo8zxBpFSTdyuvvCTG4DzXFfDfv49EMu`

SOL: `B3ZKvA3yCm5gKLb7vucU7nu8RTnp6MEAPpfeySMSMoDc`

Thank you for your support!

---

## License

MIT License
