package gauth

import (
	"net/http"
)

// NewGoogleAuthHandler creates a new http.Handler that handles Google OAuth2 authentication.
//
// It returns a http.Handler that handles the following endpoints:
//
//   - /login: Redirects to Google OAuth2 login page
//   - /callback: Handles the callback from Google OAuth2
func NewGoogleAuthHandler(storage StateStorage) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/login", NewLoginHandler(storage, "callback"))
	mux.Handle("/callback", NewCallbackHandler(storage))

	return mux
}

// Ensure our handlers implement http.Handler
var (
	_ http.Handler = (*LoginHandler)(nil)
	_ http.Handler = (*CallbackHandler)(nil)
)
