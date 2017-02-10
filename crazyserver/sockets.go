package crazyserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type outMessage struct {
	Source string                 `json:"source"`
	Data   map[string]interface{} `json:"data"`
}

type socket struct {
	socketType string
	name       string
	out        chan<- interface{}
	in         <-chan interface{}
}

type socketIndexResp struct {
	Sockets []string `json:"sockets"`
}

var socketsLock sync.Mutex
var sockets = map[string]socket{}
var socketsMaxIndex = int(0)

func socketsInitRoute(r *mux.Router) {
	r.HandleFunc("/sockets", socketsIndexHandle).Methods("GET")
	r.HandleFunc("/sockets/websocket", websocketIndexHandle).Methods("GET")
}

func socketsIndexHandle(w http.ResponseWriter, r *http.Request) {
	socketsLock.Lock()
	resp := socketIndexResp{make([]string, len(sockets))}

	i := 0
	for name, sk := range sockets {
		resp.Sockets[i] = sk.socketType + "/" + name
		i++
	}
	socketsLock.Unlock()

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(resp)
}

// Broadcast data to all active out sockets
func socketSendData(source string, data interface{}) {
	// Converting the data struct in a json compatible map
	gdata := make(map[string]interface{})
	jsondata, _ := json.Marshal(data)
	json.Unmarshal(jsondata, &gdata)

	// Send the data to all available sockets
	for _, sk := range sockets {
		sk.out <- outMessage{source, gdata}
	}
}

// TODO: function to register an handle function for in-socket data

/* Websocket implementation */
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var wsID uint

func websocketIndexHandle(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection to websocket!
	if _, ok := r.Header["Upgrade"]; ok {
		log.Println("Upgrading connection to websocket!")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		name := fmt.Sprintf("websocket%d", wsID)
		wsID++
		out := make(chan interface{}, 5)
		in := make(chan interface{}, 5)

		sk := socket{
			socketType: "websocket",
			name:       name,
			out:        out,
			in:         in,
		}

		socketsLock.Lock()
		sockets[name] = sk
		socketsLock.Unlock()

		// Out routine
		go func() {
			for {
				message := <-out
				err := conn.WriteJSON(message)
				if err != nil {
					log.Println(name, "OUT error, disconnecting!")
					conn.Close()
					socketsLock.Lock()
					if _, ok = sockets[name]; ok {
						delete(sockets, name)
					}
					socketsLock.Unlock()
					return
				}
			}
		}()
		// In routine
		go func() {
			for {
				_, msgbin, err := conn.ReadMessage()
				if err != nil {
					log.Println(name, "IN error, disconnecting!")
					conn.Close()
					socketsLock.Lock()
					if _, ok = sockets[name]; ok {
						delete(sockets, name)
					}
					socketsLock.Unlock()
					// Kill the output goroutine
					out <- nil
					return
				}

				go socketMakeRestRequest(sk, string(msgbin))
			}
		}()

		return
	}

	socketsLock.Lock()
	listLen := 0
	for name := range sockets {
		if strings.HasPrefix(name, "websocket") {
			listLen++
		}
	}

	resp := socketIndexResp{make([]string, listLen)}

	i := 0
	for name := range sockets {
		resp.Sockets[i] = name
		i++
	}
	socketsLock.Unlock()

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(resp)
}

// TODO: UDP and TCP socket implementations

type socketRestRequest struct {
	ID          *interface{} `json:"id"`
	Method      string       `json:"method"`
	Path        string       `json:"path"`
	Data        interface{}  `json:"data"`
	ContentType *string      `json:"content-type"`
}

type socketRestAnswer struct {
	ID          *interface{} `json:"id,omitempty"`
	Data        interface{}  `json:"data"`
	Code        int          `json:"code"`
	ContentType string       `json:"content-type"`
}

var rootSocketRouter *mux.Router

// Functions to interface the HTTP REST API
// This function can be lauched as a goroutine to execute the request
// The request is encapsulated in a JSON object with the followind fields:
//   - id: Optional, ID of the transaction. Useful to match request to answer. Can be any JSON value.
//   - method: HTTP method, for example "GET"
//   - path: The path to access. Example "/fleet/crazyflie0/commander"
//   - data: Data passed in the request body. Either a JSON object or a string
//   - content-type: Optional, content type. Default to "application/json; charset=UTF-8"
// The answer is a JSON object containing the fields:
//   - id: Optional, matches the request ID
//   - data: JSON object if the answer is of JSON type, string instead
//   - code: HTTP response code
//   - content-type: Content type of the answer
func socketMakeRestRequest(sk socket, request string) {
	var req socketRestRequest
	err := json.Unmarshal([]byte(request), &req)

	if err != nil || (req.Method != "GET" &&
		req.Method != "PUT" &&
		req.Method != "POST" &&
		req.Method != "DELETE") {
		sk.out <- socketRestAnswer{
			Data:        `{"error": "Invalid request format"}`,
			Code:        400,
			ContentType: "application/json; charset=UTF-8",
		}
		return
	}

	dataString, _ := json.Marshal(req.Data)

	r, _ := http.NewRequest(req.Method, req.Path, strings.NewReader(string(dataString)))
	w := newStringResponseWriter()
	rootSocketRouter.ServeHTTP(w, r)

	var body interface{}
	var contentType string
	if encoding, ok := w.Header()["Content-Type"]; ok && len(encoding) > 0 &&
		strings.Contains(strings.ToUpper(encoding[0]), "JSON") {
		json.Unmarshal([]byte(w.Body), &body)
		contentType = "application/json; charset=UTF-8"
	} else {
		body = w.Body
		contentType = "text/plain; charset=UTF-8"
	}

	sk.out <- socketRestAnswer{
		ID:          req.ID,
		Code:        w.ResponseCode,
		Data:        body,
		ContentType: contentType,
	}
}

// responseWriter that stores the restonse in a string
type stringResponseWriter struct {
	header       http.Header
	Body         string
	ResponseCode int
}

func newStringResponseWriter() *stringResponseWriter {
	return &stringResponseWriter{
		header: make(http.Header),
	}
}

func (resp *stringResponseWriter) Header() http.Header {
	return resp.header
}

func (resp *stringResponseWriter) Write(data []byte) (int, error) {
	resp.Body = resp.Body + string(data)
	return len(data), nil
}

func (resp *stringResponseWriter) WriteHeader(code int) {
	resp.ResponseCode = code
}
