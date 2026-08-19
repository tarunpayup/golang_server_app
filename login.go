package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ============================================================
// MODELS
// ============================================================

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

// ============================================================
// DEMO USERS
// ============================================================

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

// ============================================================
// DEMO SSO PROVIDERS
// ============================================================

var supportedSSOProviders = map[string]bool{
	"Google":        true,
	"Microsoft 365": true,
	"Apple ID":      true,
	"Facebook":      true,
}

// ============================================================
// SERVER / ROUTES
// ============================================================

func StartLoginServer(port string) error {

	mux := http.NewServeMux()

	// --------------------------------------------------------
	// Health
	// --------------------------------------------------------

	mux.HandleFunc("/health", HealthHandler)
	mux.HandleFunc("/hello", HelloHandler)

	// --------------------------------------------------------
	// Authentication
	// --------------------------------------------------------

	mux.HandleFunc("/api/login", LoginHandler)

	mux.HandleFunc(
		"/api/login/sso",
		SSOLoginHandler,
	)

	mux.HandleFunc(
		"/api/login/guest",
		GuestLoginHandler,
	)

	addr := "0.0.0.0:" + port

	fmt.Printf("CAOS Login API running on %s\n", addr)

	return http.ListenAndServe(addr, corsMiddleware(mux))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Max-Age", "600")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ============================================================
// PASSWORD HASHING
// ============================================================
//
// IMPORTANT:
// This SHA-256 implementation is only for development.
//
// For production authentication use:
// bcrypt or Argon2id.
//
// ============================================================

func hashPassword(password string) string {

	digest := sha256.Sum256(
		[]byte(password),
	)

	return hex.EncodeToString(
		digest[:],
	)
}

// ============================================================
// AUTHENTICATION
// ============================================================

func authenticate(
	identifier string,
	password string,
) (AuthUser, bool) {

	cleanIdentifier := strings.TrimSpace(
		identifier,
	)

	if cleanIdentifier == "" || password == "" {
		return AuthUser{}, false
	}

	hashedPassword := hashPassword(
		password,
	)

	for _, user := range authUsers {

		matchesIdentifier :=
			strings.EqualFold(
				user.Email,
				cleanIdentifier,
			) ||
				strings.EqualFold(
					user.ID,
					cleanIdentifier,
				) ||
				strings.EqualFold(
					user.Name,
					cleanIdentifier,
				)

		if !matchesIdentifier {
			continue
		}

		passwordMatches :=
			subtle.ConstantTimeCompare(
				[]byte(user.PasswordHash),
				[]byte(hashedPassword),
			) == 1

		if passwordMatches {
			return user, true
		}
	}

	return AuthUser{}, false
}

// ============================================================
// FIND USER
// ============================================================

func findUserByID(id string) (AuthUser, bool) {

	for _, user := range authUsers {

		if user.ID == id {
			return user, true
		}
	}

	return AuthUser{}, false
}

// ============================================================
// SESSION EXPIRATION
// ============================================================

func sessionExpiry(
	rememberMe bool,
) time.Time {

	duration := 24 * time.Hour

	if rememberMe {
		duration = 30 * 24 * time.Hour
	}

	return time.Now().Add(duration)
}

// ============================================================
// JSON RESPONSE
// ============================================================

func writeJSON(
	w http.ResponseWriter,
	status int,
	payload any,
) {

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(
		payload,
	)
}

// ============================================================
// LOGIN API
// ============================================================
//
// POST /api/login
//
// Request:
//
// {
//     "identifier": "partner.google@firm.com",
//     "password": "Test@123",
//     "rememberMe": true
// }
//
// ============================================================

func LoginHandler(
	w http.ResponseWriter,
	r *http.Request,
) {

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

	// Maximum request size: 1 MB

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var request LoginRequest

	err := json.NewDecoder(
		r.Body,
	).Decode(&request)

	if err != nil {

		writeJSON(
			w,
			http.StatusBadRequest,
			ErrorResponse{
				Error: "invalid JSON request",
			},
		)

		return
	}

	request.Identifier = strings.TrimSpace(
		request.Identifier,
	)

	// --------------------------------------------------------
	// Validate identifier
	// --------------------------------------------------------

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

	// --------------------------------------------------------
	// Validate password
	// --------------------------------------------------------

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

	// --------------------------------------------------------
	// Authenticate
	// --------------------------------------------------------

	user, authenticated := authenticate(
		request.Identifier,
		request.Password,
	)

	if !authenticated {

		writeJSON(
			w,
			http.StatusUnauthorized,
			ErrorResponse{
				Error: "invalid username, email, or password",
			},
		)

		return
	}

	// --------------------------------------------------------
	// Successful login
	// --------------------------------------------------------

	response := AuthResponse{
		Message: "authentication successful",

		User: user,

		SessionUntil: sessionExpiry(
			request.RememberMe,
		),
	}

	writeJSON(
		w,
		http.StatusOK,
		response,
	)
}

// ============================================================
// SSO LOGIN API
// ============================================================
//
// POST /api/login/sso
//
// Request:
//
// {
//     "provider": "Google"
// }
//
// NOTE:
// This is currently a DEMO SSO endpoint.
//
// It does NOT perform actual OAuth.
//
// ============================================================

func SSOLoginHandler(
	w http.ResponseWriter,
	r *http.Request,
) {

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

	err := json.NewDecoder(
		r.Body,
	).Decode(&request)

	if err != nil {

		writeJSON(
			w,
			http.StatusBadRequest,
			ErrorResponse{
				Error: "invalid JSON request",
			},
		)

		return
	}

	request.Provider = strings.TrimSpace(
		request.Provider,
	)

	// --------------------------------------------------------
	// Check provider
	// --------------------------------------------------------

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

	// --------------------------------------------------------
	// DEMO USER
	// --------------------------------------------------------

	user, exists := findUserByID(
		"user-1",
	)

	if !exists {

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
		Message: "demo SSO authentication successful",

		User: user,

		SessionUntil: sessionExpiry(true),
	}

	writeJSON(
		w,
		http.StatusOK,
		response,
	)
}

// ============================================================
// GUEST LOGIN API
// ============================================================
//
// POST /api/login/guest
//
// No request body required.
//
// ============================================================

func GuestLoginHandler(
	w http.ResponseWriter,
	r *http.Request,
) {

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

	// --------------------------------------------------------
	// Demo user
	// --------------------------------------------------------

	user, exists := findUserByID(
		"user-1",
	)

	if !exists {

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
		Message: "guest demo session created",

		User: user,

		SessionUntil: sessionExpiry(false),
	}

	writeJSON(
		w,
		http.StatusOK,
		response,
	)
}
