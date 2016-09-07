package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

func getControlHandler() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/reload-proxies", func(w http.ResponseWriter, req *http.Request) {
		_, err := gatewayAppMap.reload()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}).Methods("POST")

	router.HandleFunc("/reload-app-networking", func(w http.ResponseWriter, req *http.Request) {

	})

	router.HandleFunc("/apps/{appName}/macvlan", func(w http.ResponseWriter, req *http.Request) {
		appName, ok := mux.Vars(req)["appName"]
		if !ok {
			http.Error(w, "coudn't find app name in URL", http.StatusBadRequest)
			return
		}
		ip, err := getAppExternalIP(appName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write([]byte(ip))
	}).Methods("GET")

	router.HandleFunc("/apps/{appName}/macvlan", func(w http.ResponseWriter, req *http.Request) {
		appName, ok := mux.Vars(req)["appName"]
		if !ok {
			http.Error(w, "coudn't find app name in URL", http.StatusBadRequest)
			return
		}

		err := createAppInterface(appName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}).Methods("POST")

	router.HandleFunc("/apps/{appName}/macvlan", func(w http.ResponseWriter, req *http.Request) {
		appName, ok := mux.Vars(req)["appName"]
		if !ok {
			http.Error(w, "coudn't find app name in URL", http.StatusBadRequest)
			return
		}

		err := deleteAppInterface(appName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}).Methods("DELETE")

	return router
}
