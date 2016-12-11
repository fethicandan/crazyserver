package crazyserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mikehamer/crazyserver/crazyflie"
)

func commanderInitRoute(r *mux.Router) {
	r.HandleFunc("/commander", crazyflieHandleFunc(commanderSet)).Methods("PUT")
}

type commanderRequest struct {
	Roll    float32 `json:"roll"`
	Pitch   float32 `json:"pitch"`
	Yawrate float32 `json:"yawrate"`
	Thrust  uint16  `json:"thrust"`
}

func commanderSet(w http.ResponseWriter, r *http.Request, cf *crazyflie.Crazyflie) {
	var req commanderRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "Bad request!")
		return
	}

	cf.SetpointSend(req.Roll, req.Pitch, req.Yawrate, req.Thrust)

	w.Header().Set("Content-type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	fmt.Fprint(w, "{}")
}
