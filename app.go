package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type AuthUser struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	PasswordHash string `json:"-"`
}

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
	RememberMe bool   `json:"rememberMe"`
}

type SSORequest struct {
	Provider string `json:"provider"`
}

type AuthResponse struct {
	Message      string    `json:"message"`
	User         AuthUser  `json:"user"`
	SessionUntil time.Time `json:"sessionUntil"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// -----------------------------------------------------------------------------
// Demo users
// -----------------------------------------------------------------------------
// These are temporary in-memory users for development/testing.
// In production, users should come from PostgreSQL/MySQL.
var authUsers = []AuthUser{
	{
		ID:           "user-1",
		Name:         "Rajesh Kumar",
		Email:        "partner.google@firm.com",
		Role:         "Partner",
		PasswordHash: hashPassword("Test@123"),
	},
	{
		ID:           "user-2",
		Name:         "Vikram Singh",
		Email:        "vikram@firm.com",
		Role:         "Manager",
		PasswordHash: hashPassword("Test@123"),
	},
	{
		ID:           "user-3",
		Name:         "Priya Sharma",
		Email:        "priya@firm.com",
		Role:         "Staff",
		PasswordHash: hashPassword("Test@123"),
	},
	{
		ID:           "user-4",
		Name:         "Suresh Patel",
		Email:        "suresh@firm.com",
		Role:         "Staff",
		PasswordHash: hashPassword("Test@123"),
	},
}

// -----------------------------------------------------------------------------
// Demo SSO providers
// -----------------------------------------------------------------------------
// These are MOCK providers for development.
// They do NOT perform real OAuth/OpenID authentication.
var supportedSSOProviders = map[string]bool{
	"Google":        true,
	"Microsoft 365": true,
	"Apple ID":      true,
	"Facebook":      true,
}

// -----------------------------------------------------------------------------
// Password hashing
// -----------------------------------------------------------------------------
// Temporary demo implementation.
// Production should use bcrypt or Argon2id.
func hashPassword(password string) string {
	digest := sha256.Sum256([]byte(password))
	return hex.EncodeToString(digest[:])
}

// -----------------------------------------------------------------------------
// Authentication
// -----------------------------------------------------------------------------
func authenticate(identifier, password string) (AuthUser, bool) {

	cleanIdentifier := strings.TrimSpace(identifier)

	if cleanIdentifier == "" || password == "" {
		return AuthUser{}, false
	}

	hashedPassword := hashPassword(password)

	for _, user := range authUsers {

		matchesIdentifier :=
			strings.EqualFold(user.Email, cleanIdentifier) ||
				strings.EqualFold(user.ID, cleanIdentifier) ||
				strings.EqualFold(user.Name, cleanIdentifier)

		if !matchesIdentifier {
			continue
		}

		if subtle.ConstantTimeCompare(
			[]byte(user.PasswordHash),
			[]byte(hashedPassword),
		) == 1 {
			return user, true
		}
	}

	return AuthUser{}, false
}

// -----------------------------------------------------------------------------
// Find user
// -----------------------------------------------------------------------------
func findUserByID(id string) (AuthUser, bool) {

	for _, user := range authUsers {

		if user.ID == id {
			return user, true
		}
	}

	return AuthUser{}, false
}

// -----------------------------------------------------------------------------
// Session expiration
// -----------------------------------------------------------------------------
func sessionExpiry(rememberMe bool) time.Time {

	duration := 24 * time.Hour

	if rememberMe {
		duration = 30 * 24 * time.Hour
	}

	return time.Now().Add(duration)
}

// -----------------------------------------------------------------------------
// JSON response helper
// -----------------------------------------------------------------------------
func writeJSON(w http.ResponseWriter, status int, payload any) {

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}

// -----------------------------------------------------------------------------
// POST /api/login
// -----------------------------------------------------------------------------
func LoginHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {

		writeJSON(
			w,
			http.StatusMethodNotAllowed,
			ErrorResponse{
				Error: "method not allowed",
			},
		)

		return
	}

	// Limit request size to 1 MB.
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var request LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {

		writeJSON(
			w,
			http.StatusBadRequest,
			ErrorResponse{
				Error: "invalid JSON request",
			},
		)

		return
	}

	request.Identifier = strings.TrimSpace(request.Identifier)

	if request.Identifier == "" {

		writeJSON(
			w,
			http.StatusBadRequest,
			ErrorResponse{
				Error: "username or email is required",
			},
		)

		return
	}

	if request.Password == "" {

		writeJSON(
			w,
			http.StatusBadRequest,
			ErrorResponse{
				Error: "password is required",
			},
		)

		return
	}

	user, ok := authenticate(
		request.Identifier,
		request.Password,
	)

	if !ok {

		writeJSON(
			w,
			http.StatusUnauthorized,
			ErrorResponse{
				Error: "invalid username, email, or password",
			},
		)

		return
	}

	response := AuthResponse{
		Message:      "authentication successful",
		User:         user,
		SessionUntil: sessionExpiry(request.RememberMe),
	}

	writeJSON(
		w,
		http.StatusOK,
		response,
	)
}

// -----------------------------------------------------------------------------
// POST /api/login/sso
// -----------------------------------------------------------------------------
// IMPORTANT:
// This is currently a DEMO endpoint.
// It does NOT perform actual Google/Microsoft/Apple/Facebook OAuth.
func SSOLoginHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {

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

	var request SSORequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {

		writeJSON(
			w,
			http.StatusBadRequest,
			ErrorResponse{
				Error: "invalid JSON request",
			},
		)

		return
	}

	request.Provider = strings.TrimSpace(request.Provider)

	if !supportedSSOProviders[request.Provider] {

		writeJSON(
			w,
			http.StatusBadRequest,
			ErrorResponse{
				Error: "unsupported SSO provider",
			},
		)

		return
	}

	// Demo only:
	// Always return the Partner account.
	user, ok := findUserByID("user-1")

	if !ok {

		writeJSON(
			w,
			http.StatusInternalServerError,
			ErrorResponse{
				Error: "demo user not found",
			},
		)

		return
	}

	response := AuthResponse{
		Message:      "demo SSO authentication successful",
		User:         user,
		SessionUntil: sessionExpiry(true),
	}

	writeJSON(
		w,
		http.StatusOK,
		response,
	)
}

// -----------------------------------------------------------------------------
// POST /api/login/guest
// -----------------------------------------------------------------------------
func GuestLoginHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {

		writeJSON(
			w,
			http.StatusMethodNotAllowed,
			ErrorResponse{
				Error: "method not allowed",
			},
		)

		return
	}

	user, ok := findUserByID("user-1")

	if !ok {

		writeJSON(
			w,
			http.StatusInternalServerError,
			ErrorResponse{
				Error: "guest user not found",
			},
		)

		return
	}

	response := AuthResponse{
		Message:      "guest demo session created",
		User:         user,
		SessionUntil: sessionExpiry(false),
	}

	writeJSON(
		w,
		http.StatusOK,
		response,
	)
}
