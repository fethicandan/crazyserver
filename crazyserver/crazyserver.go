package crazyserver

import (
	"encoding/json"
	"fmt"
	"sync"

	"net/http"

	"github.com/mikehamer/crazyserver/crazyflie"

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

	rv1 := r.PathPrefix("/v1").Subrouter()                           // API base router
	rcf := rv1.PathPrefix("/fleet/crazyflie{id:[0-9]+}").Subrouter() // Crazyflie router

	// Initialize routes
	rv1.HandleFunc("/fleet", fleetIndexHandler).Methods("GET")
	addremoveInitRoute(rv1)
	socketsInitRoute(rv1)
	paramInitRoute(rcf)
	commanderInitRoute(rcf)

	// Optional static file server (for making standalone client)
	if len(staticPath) > 0 {
		r.PathPrefix("/static").Handler(http.StripPrefix("/static", http.FileServer(http.Dir(staticPath))))
		r.Handle("/", http.FileServer(http.Dir(staticPath)))
		r.Handle("/favicon.ico", http.FileServer(http.Dir(staticPath)))
	}

	// Export the router in a module variable to allow sockets to use it
	rootSocketRouter = r

	fmt.Println("Starting the server ...")
	fmt.Printf("Listenning on 127.0.0.1:%d\n", port)
	http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), r)
	return nil
}

// crazyflieHandleFunc returns a path handle function that decodes the Crazyflie ID from the URL, recover the Crazyflie object
// and call a path handle function with the Crazyflie object as argument.
func crazyflieHandleFunc(handleFunc func(w http.ResponseWriter, r *http.Request, cf *crazyflie.Crazyflie)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cfid := int(-1)
		fmt.Sscanf(mux.Vars(r)["id"], "%d", &cfid)

		crazyfliesLock.Lock()
		cf, ok := crazyflies[cfid]
		crazyfliesLock.Unlock()
		if ok == false {
			respondError(w, r, http.StatusNotFound, "Crazyflie not found")
			return
		}

		handleFunc(w, r, cf)
	}
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
var crazyfliesLock sync.Mutex
var crazyflies = map[int]*crazyflie.Crazyflie{}
var crazyfliesMaxIndex = int(0)
var isStarted = false
