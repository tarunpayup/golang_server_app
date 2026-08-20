package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {

	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	log.Printf(
		"CAOS server starting on port %s",
		port,
	)

	if err := StartServer(port); err != nil {
		log.Fatal(err)
	}
}

// ============================================================
// SERVER / ROUTES
// ============================================================

func StartServer(port string) error {

	mux := http.NewServeMux()

	// --------------------------------------------------------
	// Health
	// --------------------------------------------------------

	mux.HandleFunc(
		"/health",
		HealthHandler,
	)

	mux.HandleFunc(
		"/hello",
		HelloHandler,
	)

	// --------------------------------------------------------
	// Test / Database
	// --------------------------------------------------------

	mux.HandleFunc(
		"/signup",
		SignupHandler,
	)

	// --------------------------------------------------------
	// Account Registration
	// --------------------------------------------------------

	mux.HandleFunc(
		"/api/auth/register",
		CreateAccountHandler,
	)

	// --------------------------------------------------------
	// Authentication
	// --------------------------------------------------------

	mux.HandleFunc(
		"/api/login",
		LoginHandler,
	)

	mux.HandleFunc(
		"/api/login/sso",
		SSOLoginHandler,
	)

	mux.HandleFunc(
		"/api/login/guest",
		GuestLoginHandler,
	)

	// --------------------------------------------------------
	// Server
	// --------------------------------------------------------

	addr := "0.0.0.0:" + port

	fmt.Printf(
		"CAOS API server running on %s\n",
		addr,
	)

	return http.ListenAndServe(
		addr,
		corsMiddleware(mux),
	)
}

// ============================================================
// CORS
// ============================================================

func corsMiddleware(
	next http.Handler,
) http.Handler {

	return http.HandlerFunc(
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {

			w.Header().Set(
				"Access-Control-Allow-Origin",
				"*",
			)

			w.Header().Set(
				"Access-Control-Allow-Headers",
				"Content-Type, Authorization",
			)

			w.Header().Set(
				"Access-Control-Allow-Methods",
				"GET, POST, OPTIONS",
			)

			w.Header().Set(
				"Access-Control-Max-Age",
				"600",
			)

			// Handle CORS preflight
			if r.Method == http.MethodOptions {

				w.WriteHeader(
					http.StatusNoContent,
				)

				return
			}

			next.ServeHTTP(
				w,
				r,
			)
		},
	)
}
