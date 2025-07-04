package gauth

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/handlers"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type LoginHandler struct {
	stateStorage     StateStorage
	callbackEndpoint string
}

func NewLoginHandler(storage StateStorage, callbackEndpoint string) http.Handler {
	return handlers.ProxyHeaders(&LoginHandler{
		stateStorage:     storage,
		callbackEndpoint: callbackEndpoint,
	})
}

func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	redirectURI := BuildCallbackURL(r, h.callbackEndpoint)

	config := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  redirectURI,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	state, err := h.stateStorage.New(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirectURL := config.AuthCodeURL(state)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// BuildCallbackURL builds the callback URL for the given request and callback endpoint.
// For example, if the request URL is https://example.com/auth/google/login and the callback endpoint is ./callback,
// the callback URL will be https://example.com/auth/google/callback.
// It returns the callback URL as a string.
func BuildCallbackURL(r *http.Request, callbackEndpoint string) string {
	callbackEndpoint = strings.TrimPrefix(r.URL.JoinPath("..", callbackEndpoint).Path, "/")
	return fmt.Sprintf("%s://%s/%s", r.URL.Scheme, r.URL.Host, callbackEndpoint)
}

var _ http.Handler = (*LoginHandler)(nil)
