package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

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
	OrganizationID      string             `json:"organizationId"`
	OnboardingCompleted bool               `json:"onboardingCompleted"`
	SelectedCountry     CountryCodeRequest `json:"selectedCountry"`
}

type CreateAccountResponse struct {
	Success              bool              `json:"success"`
	Message              string            `json:"message"`
	UserID               string            `json:"userId"`
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

func CreateAccountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var request CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON request"})
		return
	}

	normalizeCreateAccountRequest(&request)
	if validationError := validateCreateAccountRequest(request); validationError != "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: validationError})
		return
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "DATABASE_URL is not configured"})
		return
	}
	if !strings.Contains(databaseURL, "sslmode=") {
		separator := "?"
		if strings.Contains(databaseURL, "?") {
			separator = "&"
		}
		databaseURL += separator + "sslmode=require"
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

	transaction, err := database.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "could not start account transaction"})
		return
	}

	defer transaction.Rollback()

	var existing bool
	err = transaction.QueryRowContext(
		r.Context(),
		"SELECT EXISTS (SELECT 1 FROM users WHERE username = $1 OR email = $2)",
		request.Username,
		request.Email,
	).Scan(&existing)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "could not check account uniqueness"})
		return
	}
	if existing {
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: "username or email already exists"})
		return
	}

	organizationName := request.FirmName
	organizationEmail := request.Email
	if request.Role != "Partner" {
		organizationName = "Firm (" + request.FirmRegisteredEmail + ")"
	}

	var organizationID string
	err = transaction.QueryRowContext(
		r.Context(),
		`INSERT INTO organizations (name, email, phone, gstin)
		 VALUES ($1, $2, $3, $4)
		 RETURNING org_id`,
		organizationName,
		organizationEmail,
		request.Phone,
		request.MembershipGSTIN,
	).Scan(&organizationID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "could not create organization"})
		return
	}

	userID := request.UID
	if userID == "" {
		userID = "usr_" + time.Now().Format("20060102150405.000000")
	}

	_, err = transaction.ExecContext(
		r.Context(),
		`INSERT INTO users
		 (id, organization_id, uid, name, username, email, phone, role, password, onboarding_completed)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
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
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "could not create user account"})
		return
	}

	if err := transaction.Commit(); err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "could not commit account transaction"})
		return
	}

	writeJSON(w, http.StatusCreated, CreateAccountResponse{
		Success:              true,
		Message:              "Account created successfully. Verification OTP dispatched.",
		UserID:               userID,
		RequiresVerification: true,
		OTPDispatchedTo:      request.Email,
		User: CreateAccountUser{
			UID:                 userID,
			Email:               request.Email,
			Username:            request.Username,
			Name:                request.FullName,
			Role:                request.Role,
			OnboardingCompleted: request.OnboardingCompleted,
		},
	})
}

func normalizeCreateAccountRequest(request *CreateAccountRequest) {
	request.UID = strings.TrimSpace(request.UID)
	request.FullName = strings.TrimSpace(request.FullName)
	if request.FullName == "" {
		request.FullName = strings.TrimSpace(request.Name)
	}
	request.Username = strings.ToLower(strings.TrimSpace(request.Username))
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	request.Phone = strings.TrimSpace(request.Phone)
	request.Role = strings.TrimSpace(request.Role)
	request.FirmName = strings.TrimSpace(request.FirmName)
	request.FirmRegisteredEmail = strings.ToLower(strings.TrimSpace(request.FirmRegisteredEmail))
	request.MembershipGSTIN = strings.TrimSpace(request.MembershipGSTIN)
}

func validateCreateAccountRequest(request CreateAccountRequest) string {
	if request.FullName == "" || request.Username == "" || request.Email == "" || request.Phone == "" || request.Password == "" || request.Role == "" {
		return "fullName, username, email, phone, password, and role are required"
	}
	if request.Role == "Partner" && request.FirmName == "" {
		return "firmName is required for Partner accounts"
	}
	if request.Role != "Partner" && request.FirmRegisteredEmail == "" {
		return "firmRegisteredEmail is required for non-Partner accounts"
	}
	if !validateEmailAddress(request.Email) {
		return "email must be valid"
	}
	if len(request.Password) < 8 || !containsUppercase(request.Password) || !containsLowercase(request.Password) || !containsSymbol(request.Password) || countDigits(request.Password) < 5 {
		return "password must contain an uppercase letter, a lowercase letter, a symbol, and at least 5 numbers"
	}
	return ""
}

var createAccountEmailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func validateEmailAddress(email string) bool {
	return createAccountEmailPattern.MatchString(email)
}

func containsUppercase(value string) bool {
	for _, character := range value {
		if character >= 'A' && character <= 'Z' {
			return true
		}
	}
	return false
}

func containsLowercase(value string) bool {
	for _, character := range value {
		if character >= 'a' && character <= 'z' {
			return true
		}
	}
	return false
}

func containsSymbol(value string) bool {
	for _, character := range value {
		if strings.ContainsRune(`!@#$%^&*()_+-=[]{};':"\\|,.<>/?~`+"`", character) {
			return true
		}
	}
	return false
}

func countDigits(value string) int {
	digits := 0
	for _, character := range value {
		if character >= '0' && character <= '9' {
			digits++
		}
	}
	return digits
}
