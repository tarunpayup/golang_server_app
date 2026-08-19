package main

import (
	"database/sql"
	"net/http"
	"os"

	_ "github.com/lib/pq"
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

	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "DATABASE_URL is not configured"})
		return
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "could not open database connection"})
		return
	}
	defer database.Close()

	if err := database.PingContext(r.Context()); err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "could not connect to database"})
		return
	}

	_, err = database.ExecContext(
		r.Context(),
		"INSERT INTO testpoint (id, username) VALUES ($1, $2)",
		1,
		"Tarun Bansal",
	)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "could not insert testpoint row"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "database connection successful",
		"table":   "testpoint",
		"data":    signupProbeRow{ID: 1, Username: "Tarun Bansal"},
	})
}
