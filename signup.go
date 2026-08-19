package main

import (
	"database/sql"
	"net/http"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

type signupProbeRow struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

func SignupHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)

		writeJSON(
			w,
			http.StatusMethodNotAllowed,
			ErrorResponse{
				Error: "method not allowed",
			},
		)

		return
	}

	// --------------------------------------------------------
	// Get database URL
	// --------------------------------------------------------

	databaseURL := strings.TrimSpace(
		os.Getenv("DATABASE_URL"),
	)

	if databaseURL == "" {

		writeJSON(
			w,
			http.StatusInternalServerError,
			ErrorResponse{
				Error: "DATABASE_URL is not configured",
			},
		)

		return
	}

	// --------------------------------------------------------
	// Ensure SSL is enabled
	// --------------------------------------------------------

	if !strings.Contains(
		databaseURL,
		"sslmode=",
	) {

		if strings.Contains(
			databaseURL,
			"?",
		) {
			databaseURL += "&sslmode=require"
		} else {
			databaseURL += "?sslmode=require"
		}
	}

	// --------------------------------------------------------
	// Open database
	// --------------------------------------------------------

	database, err := sql.Open(
		"postgres",
		databaseURL,
	)

	if err != nil {

		writeJSON(
			w,
			http.StatusInternalServerError,
			map[string]any{
				"error":   "could not open database connection",
				"details": err.Error(),
			},
		)

		return
	}

	defer database.Close()

	// --------------------------------------------------------
	// Test database connection
	// --------------------------------------------------------

	var result int

	err = database.QueryRowContext(
		r.Context(),
		"SELECT 1",
	).Scan(&result)

	if err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":   "could not connect to database",
				"details": err.Error(),
			},
		)

		return
	}

	// --------------------------------------------------------
	// Test testpoint table
	// --------------------------------------------------------

	var count int

	err = database.QueryRowContext(
		r.Context(),
		"SELECT COUNT(*) FROM testpoint",
	).Scan(&count)

	if err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":   "database connected, but testpoint table could not be accessed",
				"details": err.Error(),
			},
		)

		return
	}

	// --------------------------------------------------------
	// Success
	// --------------------------------------------------------

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"message":          "Supabase PostgreSQL connection successful",
			"database_test":    result,
			"testpoint_exists": true,
			"row_count":        count,
		},
	)
}
