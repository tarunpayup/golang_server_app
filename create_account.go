package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// ============================================================
// REQUEST MODELS
// ============================================================

type CountryCodeRequest struct {
	Code     string `json:"code"`
	DialCode string `json:"dialCode"`
	Name     string `json:"name"`
}

type CreateAccountRequest struct {
	UID                 string             `json:"uid"`
	FullName            string             `json:"fullName"`
	Username            string             `json:"username"`
	Email               string             `json:"email"`
	Phone               string             `json:"phone"`
	Password            string             `json:"password"`
	Role                string             `json:"role"`
	FirmName            string             `json:"firmName"`
	Name                string             `json:"name"`
	FirmRegisteredEmail string             `json:"firmRegisteredEmail"`
	MembershipGSTIN     string             `json:"membershipGstin"`
	OrganizationID      int64              `json:"organizationId"`
	OnboardingCompleted bool               `json:"onboardingCompleted"`
	SelectedCountry     CountryCodeRequest `json:"selectedCountry"`
}

// ============================================================
// RESPONSE MODELS
// ============================================================

type CreateAccountResponse struct {
	Success              bool              `json:"success"`
	Message              string            `json:"message"`
	UserID               string            `json:"userId"`
	OrganizationID       int64             `json:"organizationId"`
	RequiresVerification bool              `json:"requiresVerification"`
	OTPDispatchedTo      string            `json:"otpDispatchedTo"`
	User                 CreateAccountUser `json:"user"`
}

type CreateAccountUser struct {
	UID                 string `json:"uid"`
	Email               string `json:"email"`
	Username            string `json:"username"`
	Name                string `json:"name"`
	Role                string `json:"role"`
	OnboardingCompleted bool   `json:"onboardingCompleted"`
}

// ============================================================
// CREATE ACCOUNT HANDLER
// ============================================================

func CreateAccountHandler(w http.ResponseWriter, r *http.Request) {

	// --------------------------------------------------------
	// HTTP method
	// --------------------------------------------------------

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

	// --------------------------------------------------------
	// Limit request body
	// --------------------------------------------------------

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	// --------------------------------------------------------
	// Decode JSON
	// --------------------------------------------------------

	var request CreateAccountRequest

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

	// --------------------------------------------------------
	// Normalize request
	// --------------------------------------------------------

	normalizeCreateAccountRequest(
		&request,
	)

	// --------------------------------------------------------
	// Validate request
	// --------------------------------------------------------

	if validationError := validateCreateAccountRequest(
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

	// --------------------------------------------------------
	// DATABASE_URL
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
	// Ensure SSL
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
	// Start transaction
	// --------------------------------------------------------

	transaction, err := database.BeginTx(
		r.Context(),
		nil,
	)

	if err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":   "could not start account transaction",
				"details": err.Error(),
			},
		)

		return
	}

	defer transaction.Rollback()

	// --------------------------------------------------------
	// Check username/email uniqueness
	// --------------------------------------------------------

	var existing bool

	err = transaction.QueryRowContext(
		r.Context(),
		`
		SELECT EXISTS (
			SELECT 1
			FROM users
			WHERE username = $1
			   OR email = $2
		)
		`,
		request.Username,
		request.Email,
	).Scan(&existing)

	if err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":   "could not check account uniqueness",
				"details": err.Error(),
			},
		)

		return
	}

	if existing {

		writeJSON(
			w,
			http.StatusConflict,
			ErrorResponse{
				Error: "username or email already exists",
			},
		)

		return
	}

	// --------------------------------------------------------
	// Generate user ID
	// --------------------------------------------------------

	userID := strings.TrimSpace(
		request.UID,
	)

	if userID == "" {

		userID =
			"usr_" +
				time.Now().Format(
					"20060102150405.000000",
				)
	}

	// --------------------------------------------------------
	// Create organization
	// --------------------------------------------------------

	organizationName := request.FirmName

	organizationEmail := request.Email

	// For non-partner users, use the registered
	// firm email as the organization identity.
	if request.Role != "Partner" {

		organizationEmail =
			request.FirmRegisteredEmail

		organizationName =
			"Firm (" +
				request.FirmRegisteredEmail +
				")"
	}

	var organizationID int64

	err = transaction.QueryRowContext(
		r.Context(),
		`
		INSERT INTO organizations
			(name, email, phone, gstin)
		VALUES
			($1, $2, $3, $4)
		RETURNING org_id
		`,
		organizationName,
		organizationEmail,
		request.Phone,
		request.MembershipGSTIN,
	).Scan(&organizationID)

	if err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":   "could not create organization",
				"details": err.Error(),
			},
		)

		return
	}

	// --------------------------------------------------------
	// Create user
	// --------------------------------------------------------

	// IMPORTANT:
	// organization_id is BIGINT and receives organizationID
	// directly from organizations.org_id.

	_, err = transaction.ExecContext(
		r.Context(),
		`
		INSERT INTO users
		(
			id,
			organization_id,
			uid,
			name,
			username,
			email,
			phone,
			role,
			password,
			onboarding_completed
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
			$10
		)
		`,
		userID,
		organizationID,
		userID,
		request.FullName,
		request.Username,
		request.Email,
		request.Phone,
		request.Role,
		hashPassword(request.Password),
		request.OnboardingCompleted,
	)

	if err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":   "could not create user account",
				"details": err.Error(),
			},
		)

		return
	}

	// --------------------------------------------------------
	// Commit transaction
	// --------------------------------------------------------

	if err := transaction.Commit(); err != nil {

		writeJSON(
			w,
			http.StatusBadGateway,
			map[string]any{
				"error":   "could not commit account transaction",
				"details": err.Error(),
			},
		)

		return
	}

	// --------------------------------------------------------
	// Response
	// --------------------------------------------------------

	writeJSON(
		w,
		http.StatusCreated,
		CreateAccountResponse{
			Success: true,

			Message: "Account created successfully. Verification required.",

			UserID: userID,

			OrganizationID: organizationID,

			RequiresVerification: true,

			OTPDispatchedTo: "",

			User: CreateAccountUser{
				UID: userID,

				Email: request.Email,

				Username: request.Username,

				Name: request.FullName,

				Role: request.Role,

				OnboardingCompleted: request.OnboardingCompleted,
			},
		},
	)
}

// ============================================================
// NORMALIZE REQUEST
// ============================================================

func normalizeCreateAccountRequest(
	request *CreateAccountRequest,
) {

	request.UID =
		strings.TrimSpace(
			request.UID,
		)

	request.FullName =
		strings.TrimSpace(
			request.FullName,
		)

	if request.FullName == "" {

		request.FullName =
			strings.TrimSpace(
				request.Name,
			)
	}

	request.Username =
		strings.ToLower(
			strings.TrimSpace(
				request.Username,
			),
		)

	request.Email =
		strings.ToLower(
			strings.TrimSpace(
				request.Email,
			),
		)

	request.Phone =
		strings.TrimSpace(
			request.Phone,
		)

	request.Role =
		strings.TrimSpace(
			request.Role,
		)

	request.FirmName =
		strings.TrimSpace(
			request.FirmName,
		)

	request.FirmRegisteredEmail =
		strings.ToLower(
			strings.TrimSpace(
				request.FirmRegisteredEmail,
			),
		)

	request.MembershipGSTIN =
		strings.ToUpper(
			strings.TrimSpace(
				request.MembershipGSTIN,
			),
		)
}

// ============================================================
// VALIDATION
// ============================================================

func validateCreateAccountRequest(
	request CreateAccountRequest,
) string {

	if request.FullName == "" ||
		request.Username == "" ||
		request.Email == "" ||
		request.Phone == "" ||
		request.Password == "" ||
		request.Role == "" {

		return "fullName, username, email, phone, password, and role are required"
	}

	// --------------------------------------------------------
	// Validate role
	// --------------------------------------------------------

	switch request.Role {

	case "Partner":
		if request.FirmName == "" {
			return "firmName is required for Partner accounts"
		}

	case "Manager", "Staff":
		if request.FirmRegisteredEmail == "" {
			return "firmRegisteredEmail is required for Manager and Staff accounts"
		}

	default:
		return "role must be Partner, Manager, or Staff"
	}

	// --------------------------------------------------------
	// Validate email
	// --------------------------------------------------------

	if !validateEmailAddress(
		request.Email,
	) {

		return "email must be valid"
	}

	// --------------------------------------------------------
	// Validate firm email
	// --------------------------------------------------------

	if request.FirmRegisteredEmail != "" &&
		!validateEmailAddress(
			request.FirmRegisteredEmail,
		) {

		return "firmRegisteredEmail must be valid"
	}

	// --------------------------------------------------------
	// Validate password
	// --------------------------------------------------------

	if len(request.Password) < 8 {

		return "password must contain at least 8 characters"
	}

	if !containsUppercase(
		request.Password,
	) {

		return "password must contain an uppercase letter"
	}

	if !containsLowercase(
		request.Password,
	) {

		return "password must contain a lowercase letter"
	}

	if !containsSymbol(
		request.Password,
	) {

		return "password must contain a symbol"
	}

	if countDigits(
		request.Password,
	) < 5 {

		return "password must contain at least 5 numbers"
	}

	return ""
}

// ============================================================
// EMAIL VALIDATION
// ============================================================

var createAccountEmailPattern = regexp.MustCompile(
	`^[^\s@]+@[^\s@]+\.[^\s@]+$`,
)

func validateEmailAddress(
	email string,
) bool {

	return createAccountEmailPattern.MatchString(
		email,
	)
}

// ============================================================
// PASSWORD HELPERS
// ============================================================

func containsUppercase(
	value string,
) bool {

	for _, character := range value {

		if character >= 'A' &&
			character <= 'Z' {

			return true
		}
	}

	return false
}

func containsLowercase(
	value string,
) bool {

	for _, character := range value {

		if character >= 'a' &&
			character <= 'z' {

			return true
		}
	}

	return false
}

func containsSymbol(
	value string,
) bool {

	symbols :=
		`!@#$%^&*()_+-=[]{};':"\|,.<>/?~` + "`"

	for _, character := range value {

		if strings.ContainsRune(
			symbols,
			character,
		) {

			return true
		}
	}

	return false
}

func countDigits(
	value string,
) int {

	digits := 0

	for _, character := range value {

		if character >= '0' &&
			character <= '9' {

			digits++
		}
	}

	return digits
}
