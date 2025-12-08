package main

import (
	"encoding/json"
	"net/http"
)

func StartHTTPServer() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/", func(w http.ResponseWriter, r *http.Request) {
		allDevicesStatus := Status.GetAll()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		enc.SetIndent("", "    ")
		enc.Encode(allDevicesStatus)
	})

	return http.ListenAndServe(":8080", mux)
}
