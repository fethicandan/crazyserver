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
				var message interface{}
				err := conn.ReadJSON(message)
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
				in <- message
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
