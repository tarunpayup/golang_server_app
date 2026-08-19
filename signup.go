package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

type SignupRequest struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

func SignupHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)

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

	if !strings.Contains(databaseURL, "sslmode=") {

		if strings.Contains(databaseURL, "?") {
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

	if err := database.PingContext(
		r.Context(),
	); err != nil {

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
	// Read JSON request
	// --------------------------------------------------------

	var request SignupRequest

	err = json.NewDecoder(
		r.Body,
	).Decode(&request)

	if err != nil {

		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]any{
				"error":   "invalid JSON request",
				"details": err.Error(),
			},
		)

		return
	}

	// --------------------------------------------------------
	// Validate data
	// --------------------------------------------------------

	request.Username = strings.TrimSpace(
		request.Username,
	)

	if request.ID <= 0 {

		writeJSON(
			w,
			http.StatusBadRequest,
			ErrorResponse{
				Error: "id must be greater than 0",
			},
		)

		return
	}

	if request.Username == "" {

		writeJSON(
			w,
			http.StatusBadRequest,
			ErrorResponse{
				Error: "username is required",
			},
		)

		return
	}

	// --------------------------------------------------------
	// Insert into testpoint
	// --------------------------------------------------------

	result, err := database.ExecContext(
		r.Context(),
		`
		INSERT INTO testpoint (
			id,
			username
		)
		VALUES ($1, $2)
		`,
		request.ID,
		request.Username,
	)

	if err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":   "could not insert data into testpoint",
				"details": err.Error(),
			},
		)

		return
	}

	// --------------------------------------------------------
	// Check rows affected
	// --------------------------------------------------------

	rowsAffected, err := result.RowsAffected()

	if err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":   "could not verify inserted row",
				"details": err.Error(),
			},
		)

		return
	}

	// --------------------------------------------------------
	// Verify insertion
	// --------------------------------------------------------

	if rowsAffected != 1 {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":         "insert did not affect exactly one row",
				"rows_affected": rowsAffected,
			},
		)

		return
	}

	// --------------------------------------------------------
	// Success
	// --------------------------------------------------------

	writeJSON(
		w,
		http.StatusCreated,
		map[string]any{
			"message":       "data inserted successfully",
			"table":         "testpoint",
			"rows_affected": rowsAffected,
			"data": SignupRequest{
				ID:       request.ID,
				Username: request.Username,
			},
		},
	)
}
