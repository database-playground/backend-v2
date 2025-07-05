package gauth

import (
	"net/http"

	"github.com/database-playground/backend-v2/internal/config"
	"golang.org/x/oauth2"
)

// NewGoogleAuthHandler creates a new http.Handler that handles Google OAuth2 authentication.
//
// It returns a http.Handler that handles the following endpoints:
//
//   - /login: Redirects to Google OAuth2 login page
//   - /callback: Handles the callback from Google OAuth2
func NewGoogleAuthHandler(gauthConfig config.GAuthConfig, callbackFn func(w http.ResponseWriter, r *http.Request, token *oauth2.Token)) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/login", NewLoginHandler("callback", gauthConfig))
	mux.Handle("/callback", NewCallbackHandler(gauthConfig, callbackFn))

	return mux
}

// Ensure our handlers implement http.Handler
var (
	_ http.Handler = (*LoginHandler)(nil)
	_ http.Handler = (*CallbackHandler)(nil)
)
