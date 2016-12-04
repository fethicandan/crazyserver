package crazyserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

type fleetAddRequest struct {
	Address *string `json:"address"`
	Channel *uint8  `json:"channel"`
}

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

func fleetAddHandler(w http.ResponseWriter, r *http.Request) {
	var req fleetAddRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Address == nil {
		respondError(w, r, http.StatusBadRequest, "Bad request!")
		return
	}

	channel := *req.Channel
	address := uint64(0)
	if req.Address != nil {
		fmt.Sscanf(*req.Address, "%x", &address)
		if address == 0 || len(*req.Address) != 10 {
			respondError(w, r, http.StatusBadRequest, "Bad request! Address invalid")
			return
		}
	} else {
		address = 0xe7e7e7e7e7
	}

	crazyfliesLock.Lock()
	cfid, err := AddCrazyflie(address, channel)
	crazyfliesLock.Unlock()

	if err != nil {
		str := fmt.Sprintf("Cannot connect to Crazyflie: %q", err)
		respondError(w, r, http.StatusNotFound, str)
		return
	}

	w.Header().Set("Content-type", "application/json; charset=UTF-8")
	w.Header().Set("Location", fmt.Sprintf("/fleet/crazyflie%d", cfid))
	w.WriteHeader(http.StatusOK)
}

func fleetRemoveHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cfid := int(-1)
	fmt.Sscanf(vars["id"], "%d", &cfid)

	crazyfliesLock.Lock()
	err := RemoveCrazyflie(cfid)
	crazyfliesLock.Unlock()

	if err != nil {
		respondError(w, r, http.StatusNotFound, fmt.Sprint(err))
		return
	}

	w.Header().Set("Content-type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
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
var crazyfliesLock sync.Mutex
var crazyflies = map[int]*crazyflie.Crazyflie{}
var crazyfliesMaxIndex = int(0)
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

func AddCrazyflie(address uint64, channel uint8) (int, error) {
	if !isStarted {
		err := Start()
		if err != nil {
			return -1, err
		}
	}

	// connect to the crazyflie
	cf, err := crazyflie.Connect(radio, address, channel)
	if err != nil {
		log.Printf("Error adding crazyflie: %s", err)
		return -1, err
	}

	// do other management stuff
	//...

	// Add to the list and return the index
	crazyflies[crazyfliesMaxIndex] = cf
	crazyfliesMaxIndex += 1
	return crazyfliesMaxIndex - 1, nil
}

func RemoveCrazyflie(cfid int) error {
	if _, ok := crazyflies[cfid]; ok == false {
		return errors.New(fmt.Sprintf("Crazyflie %d not found!", cfid))
	}

	crazyflies[cfid].DisconnectImmediately()
	delete(crazyflies, cfid)

	return nil
}
