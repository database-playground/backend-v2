package gauth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/database-playground/backend-v2/internal/authutil"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/gorilla/handlers"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type LoginHandler struct {
	gauthConfig config.GAuthConfig

	stateStorage         StateStorage
	callbackRelativePath string
}

func NewLoginHandler(storage StateStorage, callbackRelativePath string, gauthConfig config.GAuthConfig) http.Handler {
	return handlers.ProxyHeaders(&LoginHandler{
		gauthConfig:          gauthConfig,
		stateStorage:         storage,
		callbackRelativePath: callbackRelativePath,
	})
}

func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	googleOAuthRedirectURI := BuildCallbackURL(r, h.callbackRelativePath)

	config := &oauth2.Config{
		ClientID:     h.gauthConfig.ClientID,
		ClientSecret: h.gauthConfig.ClientSecret,
		RedirectURL:  googleOAuthRedirectURI,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	state := r.URL.Query().Get("state")

	// https://www.ietf.org/archive/id/draft-ietf-oauth-security-topics-22.html#name-countermeasures-6
	//
	// If state is used for carrying application state, and integrity of its contents is a concern, clients
	// MUST protect state against tampering and swapping. This can be achieved by binding the contents of
	// state to the browser session and/or signed/encrypted state values as discussed in the now-expired draft.
	//
	// The state is set by the client, so I don't think we need to encrypt it.
	http.SetCookie(w, &http.Cookie{
		// https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-rfc6265bis-20#section-4.1.3.2
		//
		// If a cookie's name begins with a case-sensitive match for the string __Host-,
		// then the cookie will have been set with a Secure attribute, a Path attribute with
		// a value of /, and no Domain attribute.
		Name:     "__Host-Gauth-State",
		Value:    state,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	verifier, err := authutil.GenerateToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirectURL := config.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.S256ChallengeOption(verifier))
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
