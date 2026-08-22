package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"

	_ "github.com/lib/pq"
)

// ============================================================
// REQUEST / RESPONSE MODELS
// ============================================================

type OfficeStoreRequest struct {
	Offices []OfficeStoreRecord `json:"offices"`
}

type OfficeStoreRecord struct {
	OrganizationID int64  `json:"organization_id"`
	OfficeName     string `json:"office_name"`
	OfficeType     string `json:"office_type"`
	BranchRep      string `json:"branch_rep"`
	AddressOne     string `json:"address_one"`
	AddressTwo     string `json:"address_two"`
	AddressThree   string `json:"address_three"`
	City           string `json:"city"`
	State          string `json:"state"`
	Country        string `json:"country"`
	Pincode        string `json:"pincode"`
	Mobile         string `json:"mobile"`
	Phone          string `json:"phone"`
}

type OfficeStoreResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

type OrganizationLookupResponse struct {
	Success        bool  `json:"success"`
	OrganizationID int64 `json:"organizationId"`
}

// ============================================================
// ORGANIZATION LOOKUP HANDLER
// ============================================================

func OrganizationByEmailHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {

		w.Header().Set(
			"Allow",
			http.MethodGet,
		)

		writeJSON(
			w,
			http.StatusMethodNotAllowed,
			ErrorResponse{
				Error: "method not allowed",
			},
		)

		return
	}

	email := strings.TrimSpace(
		r.URL.Query().Get("email"),
	)

	if email == "" {

		writeJSON(
			w,
			http.StatusBadRequest,
			ErrorResponse{
				Error: "email is required",
			},
		)

		return
	}

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

	var organizationID int64

	err = database.QueryRowContext(
		r.Context(),
		`
		SELECT o.org_id
		FROM organizations o
		LEFT JOIN users u
			ON u.organization_id = o.org_id
		WHERE LOWER(u.email) = LOWER($1)
		   OR LOWER(o.email) = LOWER($1)
		ORDER BY
			CASE
				WHEN LOWER(u.email) = LOWER($1) THEN 0
				ELSE 1
			END
		LIMIT 1
		`,
		email,
	).Scan(&organizationID)

	if err == sql.ErrNoRows {

		writeJSON(
			w,
			http.StatusNotFound,
			ErrorResponse{
				Error: "organization not found for email",
			},
		)

		return
	}

	if err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":   "could not fetch organization",
				"details": err.Error(),
			},
		)

		return
	}

	writeJSON(
		w,
		http.StatusOK,
		OrganizationLookupResponse{
			Success:        true,
			OrganizationID: organizationID,
		},
	)
}

// ============================================================
// OFFICE STORE HANDLER
// ============================================================

func OfficeStoreHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {

		w.Header().Set(
			"Allow",
			http.MethodPost,
		)

		writeJSON(
			w,
			http.StatusMethodNotAllowed,
			ErrorResponse{
				Error: "method not allowed",
			},
		)

		return
	}

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var request OfficeStoreRequest

	if err := json.NewDecoder(
		r.Body,
	).Decode(&request); err != nil {

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

	normalizeOfficeStoreRequest(
		&request,
	)

	if validationError := validateOfficeStoreRequest(
		request,
	); validationError != "" {

		writeJSON(
			w,
			http.StatusBadRequest,
			ErrorResponse{
				Error: validationError,
			},
		)

		return
	}

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

	transaction, err := database.BeginTx(
		r.Context(),
		nil,
	)

	if err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":   "could not start office transaction",
				"details": err.Error(),
			},
		)

		return
	}

	defer transaction.Rollback()

	organizationID := request.Offices[0].OrganizationID

	_, err = transaction.ExecContext(
		r.Context(),
		`
		DELETE FROM office_dit
		WHERE organization_id = $1
		`,
		organizationID,
	)

	if err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":           "could not clear existing office information",
				"organization_id": organizationID,
				"details":         err.Error(),
			},
		)

		return
	}

	for _, office := range request.Offices {

		_, err = transaction.ExecContext(
			r.Context(),
			`
			INSERT INTO office_dit
			(
				organization_id,
				office_name,
				office_type,
				branch_rep,
				address_one,
				address_two,
				address_three,
				city,
				state,
				country,
				pincode,
				mobile,
				phone
			)
			VALUES
			(
				$1,
				$2,
				$3,
				$4,
				$5,
				$6,
				$7,
				$8,
				$9,
				$10,
				$11,
				$12,
				$13
			)
			`,
			office.OrganizationID,
			office.OfficeName,
			office.OfficeType,
			office.BranchRep,
			office.AddressOne,
			office.AddressTwo,
			office.AddressThree,
			office.City,
			office.State,
			office.Country,
			office.Pincode,
			office.Mobile,
			office.Phone,
		)

		if err != nil {

			writeJSON(
				w,
				http.StatusBadGateway,
				map[string]any{
					"error":       "could not store office information",
					"office_name": office.OfficeName,
					"details":     err.Error(),
				},
			)

			return
		}
	}

	if err := transaction.Commit(); err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":   "could not commit office transaction",
				"details": err.Error(),
			},
		)

		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		OfficeStoreResponse{
			Success: true,
			Message: "Office information stored successfully",
			Count:   len(request.Offices),
		},
	)
}

// ============================================================
// NORMALIZE / VALIDATE
// ============================================================

func normalizeOfficeStoreRequest(request *OfficeStoreRequest) {

	phoneCleaner := regexp.MustCompile(`[^\d+]`)

	for index := range request.Offices {

		office := &request.Offices[index]

		office.OfficeName = strings.TrimSpace(
			office.OfficeName,
		)

		office.OfficeType = strings.TrimSpace(
			office.OfficeType,
		)

		office.BranchRep = strings.TrimSpace(
			office.BranchRep,
		)

		office.AddressOne = strings.TrimSpace(
			office.AddressOne,
		)

		office.AddressTwo = strings.TrimSpace(
			office.AddressTwo,
		)

		office.AddressThree = strings.TrimSpace(
			office.AddressThree,
		)

		office.City = strings.TrimSpace(
			office.City,
		)

		office.State = strings.TrimSpace(
			office.State,
		)

		office.Country = strings.TrimSpace(
			office.Country,
		)

		office.Pincode = strings.TrimSpace(
			office.Pincode,
		)

		office.Mobile = phoneCleaner.ReplaceAllString(
			strings.TrimSpace(
				office.Mobile,
			),
			"",
		)

		office.Phone = phoneCleaner.ReplaceAllString(
			strings.TrimSpace(
				office.Phone,
			),
			"",
		)
	}
}

func validateOfficeStoreRequest(request OfficeStoreRequest) string {

	if len(request.Offices) == 0 {
		return "at least one office is required"
	}

	organizationID := request.Offices[0].OrganizationID

	for _, office := range request.Offices {

		if office.OrganizationID <= 0 {
			return "organization_id is required"
		}

		if office.OrganizationID != organizationID {
			return "all offices must belong to the same organization_id"
		}

		if office.OfficeName == "" {
			return "office_name is required"
		}

		if len(office.OfficeName) > 50 {
			return "office_name must be 50 characters or fewer"
		}

		if office.OfficeType == "" {
			return "office_type is required"
		}

		if len(office.OfficeType) > 50 {
			return "office_type must be 50 characters or fewer"
		}

		if len(office.BranchRep) > 50 {
			return "branch_rep must be 50 characters or fewer"
		}

		if office.AddressOne == "" {
			return "address_one is required"
		}

		if office.City == "" {
			return "city is required"
		}

		if len(office.City) > 20 {
			return "city must be 20 characters or fewer"
		}

		if len(office.State) > 20 {
			return "state must be 20 characters or fewer"
		}

		if len(office.Country) > 20 {
			return "country must be 20 characters or fewer"
		}

		if office.Pincode == "" {
			return "pincode is required"
		}

		if len(office.Pincode) > 6 {
			return "pincode must be 6 characters or fewer"
		}

		if len(office.Mobile) > 15 {
			return "mobile must be 15 characters or fewer"
		}

		if len(office.Phone) > 15 {
			return "phone must be 15 characters or fewer"
		}
	}

	return ""
}
