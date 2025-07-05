package gauth

import (
	"net/http"
	"net/url"

	"github.com/database-playground/backend-v2/internal/authutil"
	"github.com/gorilla/handlers"
	"golang.org/x/oauth2"
)

const stateValue = "__triggered_by_gauth_login"
const verifierCookieName = "Gauth-Verifier"

type LoginHandler struct {
	oauthConfig *oauth2.Config
}

func NewLoginHandler(oauthConfig *oauth2.Config) http.Handler {
	return handlers.ProxyHeaders(&LoginHandler{
		oauthConfig: oauthConfig,
	})
}

func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	verifier, err := authutil.GenerateToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	callbackURL, err := url.Parse(h.oauthConfig.RedirectURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     verifierCookieName,
		Value:    verifier,
		Path:     callbackURL.Path,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 60, // 30 min
	})

	redirectURL := h.oauthConfig.AuthCodeURL(
		stateValue,
		oauth2.AccessTypeOnline,
		oauth2.S256ChallengeOption(verifier),
	)

	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

var _ http.Handler = (*LoginHandler)(nil)
