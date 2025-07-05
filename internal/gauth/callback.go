package gauth

import (
	"net/http"

	"github.com/database-playground/backend-v2/internal/config"
	"github.com/gorilla/handlers"
	"golang.org/x/oauth2"
)

type CallbackHandler struct {
	gauthConfig config.GAuthConfig
	callbackFn  func(w http.ResponseWriter, r *http.Request, token *oauth2.Token)
}

func NewCallbackHandler(gauthConfig config.GAuthConfig, callbackFn func(w http.ResponseWriter, r *http.Request, token *oauth2.Token)) http.Handler {
	return handlers.ProxyHeaders(&CallbackHandler{gauthConfig: gauthConfig, callbackFn: callbackFn})
}

func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state != stateValue {
		http.Error(w, "Invalid state. Please re-initiate the login process.", http.StatusBadRequest)
		return
	}

	verifier, err := r.Cookie(verifierCookieName)
	if err != nil {
		http.Error(w, "No verifier cookie found. Please re-initiate the login process.", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	oauthConfig := BuildOAuthConfig(h.gauthConfig)

	token, err := oauthConfig.Exchange(r.Context(), code, oauth2.S256ChallengeOption(verifier.Value))
	if err != nil {
		http.Error(w, "Failed to exchange code for token", http.StatusInternalServerError)
		return
	}

	h.callbackFn(w, r, token)
}

var _ http.Handler = (*CallbackHandler)(nil)
