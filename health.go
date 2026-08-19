package main

import "net/http"

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Health-Status", "online")
		w.WriteHeader(http.StatusOK)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "online"})
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "hello world"})
}
