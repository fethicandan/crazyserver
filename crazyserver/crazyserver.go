package crazyserver

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

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

	r.HandleFunc("/fleet", fleetIndexHandler).Methods("GET")
	r.HandleFunc("/fleet", fleetAddHandler).Methods("POST")
	r.HandleFunc("/fleet/crazyflie{id:[0-9]+}", fleetRemoveHandler).Methods("DELETE")

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

type errorResponse struct {
	Error string `json:"error"`
}

func fleetIndexHandler(w http.ResponseWriter, r *http.Request) {
	connected := fleetIndexResponse{
		[]string{},
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(connected)
}

func fleetAddHandler(w http.ResponseWriter, r *http.Request) {
	respondError(w, r, http.StatusBadRequest, "Cannot create more Crazyflie connection!")
}

func fleetRemoveHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := uint(0)
	fmt.Sscanf("%d", vars["id"], &id)

	respondError(w, r, http.StatusNotFound, fmt.Sprintf("Crazyflie with id %d not found!", id))
}

func respondError(w http.ResponseWriter, r *http.Request, httpStatus int, msg string) {
	resp := errorResponse{
		Error: msg,
	}

	w.Header().Set("Content-type", "application/json; charset=UTF-8")
	w.WriteHeader(httpStatus)

	json.NewEncoder(w).Encode(resp)
}

var radio *crazyradio.RadioDevice
var crazyflies = map[uint64]*crazyflie.Crazyflie{}
var isStarted = false

func Start() error {
	var err error
	radio, err = crazyradio.Open()
	if err != nil {
		return err
	}

	isStarted = true
	return nil
}

func Stop() {
	for _, v := range crazyflies {
		v.DisconnectImmediately()
	}
	radio.Close()
}

func AddCrazyflie(address uint64, channel uint8) (*crazyflie.Crazyflie, error) {
	if !isStarted {
		Start()
	}

	// connect to the crazyflie
	cf, err := crazyflie.Connect(radio, address, channel)
	if err != nil {
		log.Printf("Error adding crazyflie: %s", err)
		return nil, err
	}

	// do other management stuff
	//...

	crazyflies[address] = cf
	return cf, nil
}

func BeginLogging(address uint64, variables []string, period time.Duration) (int, error) {
	cf, ok := crazyflies[address]
	if !ok {
		return -1, fmt.Errorf("No crazyflie with address %X found", address) // TODO: replace with actual error
	}

	blockid, err := cf.LogBlockAdd(period, variables)
	if err != nil {
		return -1, err
	}

	err = cf.LogBlockStart(blockid)
	if err != nil {
		return -1, err
	}

	return blockid, nil
}

func StopLogging(address uint64, blockid int) error {
	cf, ok := crazyflies[address]
	if !ok {
		return fmt.Errorf("No crazyflie with address %X found", address) // TODO: replace with actual error
	}

	err := cf.LogBlockStop(blockid)
	if err != nil {
		return err
	}

	return nil
}
