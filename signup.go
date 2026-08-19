package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type signupProbeRow struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

func SignupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	supabaseURL := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Supabase environment variables are not configured"})
		return
	}

	payload, err := json.Marshal(signupProbeRow{ID: 1, Username: "Tarun Bansal"})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "could not prepare database request"})
		return
	}

	request, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodPost,
		fmt.Sprintf("%s/rest/v1/testpoint", supabaseURL),
		bytes.NewReader(payload),
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "could not create database request"})
		return
	}

	request.Header.Set("apikey", supabaseKey)
	request.Header.Set("Authorization", "Bearer "+supabaseKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Prefer", "return=representation")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "could not connect to Supabase"})
		return
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: fmt.Sprintf("Supabase returned HTTP %d", response.StatusCode)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "database connection successful",
		"table":   "testpoint",
		"data":    signupProbeRow{ID: 1, Username: "Tarun Bansal"},
	})
}
