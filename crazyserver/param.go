package crazyserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mikehamer/crazyserver/crazyflie"
)

func paramInitRoute(r *mux.Router) {
	r.HandleFunc("/param/toc", crazyflieHandleFunc(paramTocIndex)).Methods("GET")
	r.HandleFunc("/param/params", crazyflieHandleFunc(paramIndex)).Methods("GET")
	r.HandleFunc("/param/params/{group}/{name}", crazyflieHandleFunc(paramAccess)).Methods("GET", "PUT")
}

func convertToFloat(val interface{}) float64 {
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
	return fval
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
		resp.Params[name] = convertToFloat(val)
	}

	w.Header().Set("Content-type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(resp)
}

type paramTocItem struct {
	Group  string `json:"group"`
	Name   string `json:"name"`
	Access string `json:"access"`
}

type paramTocIndexResponse struct {
	Toc []paramTocItem `json:"toc"`
}

func paramTocIndex(w http.ResponseWriter, r *http.Request, cf *crazyflie.Crazyflie) {
	resp := paramTocIndexResponse{}

	tocList := cf.ParamGetToc()

	resp.Toc = make([]paramTocItem, len(tocList))

	for i, tocItem := range tocList {
		resp.Toc[i].Group = tocItem.Group
		resp.Toc[i].Name = tocItem.Name
		resp.Toc[i].Access = tocItem.Access
	}

	w.Header().Set("Content-type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(resp)
}

type paramAccessFormat struct {
	Group string  `json:"group"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

func paramAccess(w http.ResponseWriter, r *http.Request, cf *crazyflie.Crazyflie) {
	vars := mux.Vars(r)
	group, name := vars["group"], vars["name"]

	if r.Method == "PUT" {
		var req paramAccessFormat

		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			respondError(w, r, http.StatusBadRequest, "Bad request!")
			return
		}

		err = cf.ParamWriteFromFloat64(fmt.Sprintf("%s.%s", group, name), req.Value)

		if err != nil {
			respondError(w, r, http.StatusBadRequest, fmt.Sprint(err))
		}
	}

	val, err := cf.ParamRead(fmt.Sprintf("%s.%s", group, name))

	if err != nil {
		respondError(w, r, http.StatusNotFound, fmt.Sprint(err))
		return
	}

	resp := paramAccessFormat{
		Group: group,
		Name:  name,
		Value: convertToFloat(val),
	}

	w.Header().Set("Content-type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(resp)
}
