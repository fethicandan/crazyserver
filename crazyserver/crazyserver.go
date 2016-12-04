package crazyserver

import (
	"encoding/json"
	"fmt"
	"sync"

	"net/http"

	"github.com/mikehamer/crazyserver/crazyflie"
	"github.com/mikehamer/crazyserver/crazyradio"

	"github.com/gorilla/mux"
	"github.com/urfave/cli"
)

var ServeCommand cli.Command = cli.Command{
	Name:   "serve",
	Usage:  "Start the HTTP/REST server",
	Action: serveCommandHandler,
	Flags: []cli.Flag{
		cli.UintFlag{
			Name:  "port, p",
			Value: 8000,
			Usage: "HTTP Listenning port",
		},
		cli.StringFlag{
			Name:  "static, s",
			Value: "",
			Usage: "Optional static folder. Served on /static with index.html accessible on /",
		},
	},
}

func serveCommandHandler(ctx *cli.Context) error {
	port := ctx.Uint("port")
	staticPath := ctx.String("static")

	r := mux.NewRouter()

	// Initialize routes
	r.HandleFunc("/fleet", fleetIndexHandler).Methods("GET")
	addremoveInitRoute(r)

	// Optional static file server (for making standalone client)
	if len(staticPath) > 0 {
		r.PathPrefix("/static").Handler(http.StripPrefix("/static", http.FileServer(http.Dir(staticPath))))
		r.Handle("/", http.FileServer(http.Dir(staticPath)))
		r.Handle("/favicon.ico", http.FileServer(http.Dir(staticPath)))
	}

	fmt.Println("Starting the server ...")
	fmt.Printf("Listenning on 127.0.0.1:%d\n", port)
	http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), r)
	return nil
}

type fleetIndexResponse struct {
	Connected []string `json:"connected"`
}

// fleetIndexHandler sends a list of connected Crazyflie to the client.
func fleetIndexHandler(w http.ResponseWriter, r *http.Request) {
	response := fleetIndexResponse{}

	crazyfliesLock.Lock()
	response.Connected = make([]string, len(crazyflies))

	i := 0
	for cfid, _ := range crazyflies {
		response.Connected[i] = fmt.Sprintf("crazyflie%d", cfid)
	}
	crazyfliesLock.Unlock()

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(response)
}

type errorResponse struct {
	Error string `json:"error"`
}

// respondError utility function to send back an error with an explanation string.
func respondError(w http.ResponseWriter, r *http.Request, httpStatus int, msg string) {
	resp := errorResponse{
		Error: msg,
	}

	w.Header().Set("Content-type", "application/json; charset=UTF-8")
	w.WriteHeader(httpStatus)

	json.NewEncoder(w).Encode(resp)
}

// This is the main state of the server. Currently the server is using one Crazyradio to connect a list of Crazyflies.
// The lock should be aquired by anyone accessing the list.
var radio *crazyradio.RadioDevice
var crazyfliesLock sync.Mutex
var crazyflies = map[int]*crazyflie.Crazyflie{}
var crazyfliesMaxIndex = int(0)
var isStarted = false
