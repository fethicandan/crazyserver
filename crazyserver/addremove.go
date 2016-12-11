package crazyserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mikehamer/crazyserver/crazyflie"
)

func addremoveInitRoute(r *mux.Router) {
	r.HandleFunc("/fleet", fleetAddHandler).Methods("POST")
	r.HandleFunc("/fleet/crazyflie{id:[0-9]+}", fleetRemoveHandler).Methods("DELETE")
}

type fleetAddRequest struct {
	Address *string `json:"address"`
	Channel *uint8  `json:"channel"`
}

type fleetAddResponse struct {
	Location string `json:"location"`
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
	w.Header().Set("Location", fmt.Sprintf("/v1/fleet/crazyflie%d", cfid))
	w.WriteHeader(http.StatusOK)

	resp := fleetAddResponse{Location: fmt.Sprintf("/v1/fleet/crazyflie%d", cfid)}
	json.NewEncoder(w).Encode(resp)
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

	fmt.Fprint(w, "{}")
}

// Low utility functions to add and remove Crazyflie from the server

// Start opens the Crazyradio
func Start() error {
	//crazyradio already opened by main, do nothing
	return nil
}

// Stop closes all connected Crazyflie and closes the Crazyradio
func Stop() {
	for _, v := range crazyflies {
		v.DisconnectImmediately()
	}
}

// AddCrazyflie connects to a Crazyfle at address and channel and add it to the crazyflie list.
// Returns the index of the connected Crazyflie.
func AddCrazyflie(address uint64, channel uint8) (int, error) {
	if !isStarted {
		err := Start()
		if err != nil {
			return -1, err
		}
	}

	// connect to the crazyflie
	cf, err := crazyflie.Connect(address, channel)
	if err != nil {
		log.Printf("Error adding crazyflie: %s", err)
		return -1, err
	}

	cf.ParamTOCGetList()
	// do other management stuff
	//...

	// Add to the list and return the index
	crazyflies[crazyfliesMaxIndex] = cf
	crazyfliesMaxIndex += 1
	return crazyfliesMaxIndex - 1, nil
}

// RemoveCrazyflie disconnect the copter at index cfid and remove it from the list of Crazyflie.
func RemoveCrazyflie(cfid int) error {
	if _, ok := crazyflies[cfid]; ok == false {
		return errors.New(fmt.Sprintf("Crazyflie %d not found!", cfid))
	}

	crazyflies[cfid].DisconnectImmediately()
	delete(crazyflies, cfid)

	return nil
}
