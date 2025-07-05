package gauth

import (
	"net/http"
	"net/url"

	"github.com/gorilla/handlers"
	"golang.org/x/oauth2"
)

type CallbackHandler struct {
	oauthConfig *oauth2.Config
	callbackFn  func(w http.ResponseWriter, r *http.Request, token *oauth2.Token)
}

func NewCallbackHandler(oauthConfig *oauth2.Config, callbackFn func(w http.ResponseWriter, r *http.Request, token *oauth2.Token)) http.Handler {
	return handlers.ProxyHeaders(&CallbackHandler{
		oauthConfig: oauthConfig,
		callbackFn:  callbackFn,
	})
}

func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state != stateValue {
		http.Error(w, "Invalid state. Please re-initiate the login process.", http.StatusBadRequest)
		return
	}

	verifier, err := r.Cookie(verifierCookieName)
	if err != nil || verifier.Value == "" {
		http.Error(w, "No verifier cookie found. Please re-initiate the login process.", http.StatusBadRequest)
		return
	}

	callbackURL, err := url.Parse(h.oauthConfig.RedirectURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     verifierCookieName,
		Value:    "",
		Path:     callbackURL.Path,
		MaxAge:   -1,
		HttpOnly: true,
	})

	code := r.URL.Query().Get("code")
	token, err := h.oauthConfig.Exchange(r.Context(), code, oauth2.VerifierOption(verifier.Value))
	if err != nil {
		http.Error(w, "Failed to exchange code for token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.callbackFn(w, r, token)
}

var _ http.Handler = (*CallbackHandler)(nil)
