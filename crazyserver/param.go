package crazyserver

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mikehamer/crazyserver/crazyflie"
)

func paramInitRoute(r *mux.Router) {
	r.HandleFunc("/fleet/crazyflie{id:[0-9]+}/param", crazyflieHandleFunc(paramIndex)).Methods("GET")
}

type paramIndexResponse struct {
	Params map[string]float64 `json:"params"`
}

func paramIndex(w http.ResponseWriter, r *http.Request, cf *crazyflie.Crazyflie) {
	resp := paramIndexResponse{}
	resp.Params = make(map[string]float64)

	paramNames := cf.ParamGetList()
	for _, name := range paramNames {
		val, _ := cf.ParamRead(name)
		fval := float64(0.0)
		switch v := val.(type) {
		case uint8:
			fval = float64(v)
		case uint16:
			fval = float64(v)
		case uint32:
			fval = float64(v)
		case int8:
			fval = float64(v)
		case int16:
			fval = float64(v)
		case int32:
			fval = float64(v)
		case float32:
			fval = float64(v)
		default:
			fval = -42
		}
		resp.Params[name] = fval
	}

	w.Header().Set("Content-type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(resp)
}
