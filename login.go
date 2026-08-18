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

var authUsers = []AuthUser{
	{ID: "user-1", Name: "Rajesh Kumar", Email: "partner.google@firm.com", Role: "Partner", PasswordHash: hashPassword("Test@123")},
	{ID: "user-2", Name: "Vikram Singh", Email: "vikram@firm.com", Role: "Manager", PasswordHash: hashPassword("Test@123")},
	{ID: "user-3", Name: "Priya Sharma", Email: "priya@firm.com", Role: "Staff", PasswordHash: hashPassword("Test@123")},
	{ID: "user-4", Name: "Suresh Patel", Email: "suresh@firm.com", Role: "Staff", PasswordHash: hashPassword("Test@123")},
}

var supportedSSOProviders = map[string]bool{
	"Google":        true,
	"Microsoft 365": true,
	"Apple ID":      true,
	"Facebook":      true,
}

func hashPassword(password string) string {
	digest := sha256.Sum256([]byte(password))
	return hex.EncodeToString(digest[:])
}

func authenticate(identifier, password string) (AuthUser, bool) {
	cleanIdentifier := strings.TrimSpace(identifier)
	for _, user := range authUsers {
		matchesIdentifier := strings.EqualFold(user.Email, cleanIdentifier) ||
			strings.EqualFold(user.ID, cleanIdentifier) ||
			strings.EqualFold(user.Name, cleanIdentifier)
		if matchesIdentifier && subtle.ConstantTimeCompare([]byte(user.PasswordHash), []byte(hashPassword(password))) == 1 {
			return user, true
		}
	}
	return AuthUser{}, false
}

func findUserByID(id string) (AuthUser, bool) {
	for _, user := range authUsers {
		if user.ID == id {
			return user, true
		}
	}
	return AuthUser{}, false
}

func sessionExpiry(rememberMe bool) time.Time {
	duration := 24 * time.Hour
	if rememberMe {
		duration = 30 * 24 * time.Hour
	}
	return time.Now().Add(duration)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var request LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return
	}
	if strings.TrimSpace(request.Identifier) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username or email is required"})
		return
	}

	user, ok := authenticate(request.Identifier, request.Password)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username, email, or password"})
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{
		Message:      "authentication successful",
		User:         user,
		SessionUntil: sessionExpiry(request.RememberMe),
	})
}

func SSOLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var request SSORequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return
	}
	if !supportedSSOProviders[request.Provider] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported SSO provider"})
		return
	}

	user, _ := findUserByID("user-1")
	writeJSON(w, http.StatusOK, AuthResponse{
		Message:      "SSO authentication successful",
		User:         user,
		SessionUntil: sessionExpiry(true),
	})
}

func GuestLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	user, _ := findUserByID("user-1")
	writeJSON(w, http.StatusOK, AuthResponse{
		Message:      "guest demo session created",
		User:         user,
		SessionUntil: sessionExpiry(false),
	})
}
