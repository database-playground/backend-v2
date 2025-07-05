package gauth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/database-playground/backend-v2/internal/authutil"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/gorilla/handlers"
	"golang.org/x/oauth2"
)

const stateValue = "__triggered_by_gauth_login"
const verifierCookieName = "__Host-Gauth-Verifier"

type LoginHandler struct {
	gauthConfig config.GAuthConfig

	callbackRelativePath string
}

func NewLoginHandler(callbackRelativePath string, gauthConfig config.GAuthConfig) http.Handler {
	return handlers.ProxyHeaders(&LoginHandler{
		gauthConfig:          gauthConfig,
		callbackRelativePath: callbackRelativePath,
	})
}

func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	googleOAuthRedirectURI := BuildCallbackURL(r, h.callbackRelativePath)
	oauthConfig := BuildOAuthConfig(h.gauthConfig)
	oauthConfig.RedirectURL = googleOAuthRedirectURI

	verifier, err := authutil.GenerateToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     verifierCookieName,
		Value:    verifier,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	redirectURL := oauthConfig.AuthCodeURL(
		stateValue,
		oauth2.AccessTypeOnline,
		oauth2.S256ChallengeOption(verifier),
	)

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
