package authservice

import (
	"net/http"
	"net/url"

	"github.com/database-playground/backend-v2/internal/authutil"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleoauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

const verifierCookieName = "Gauth-Verifier"

// BuildOAuthConfig builds an oauth2.Config from a gauthConfig.
func BuildOAuthConfig(gauthConfig config.GAuthConfig) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     gauthConfig.ClientID,
		ClientSecret: gauthConfig.ClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

type GauthHandler struct {
	oauthConfig *oauth2.Config
}

func NewGauthHandler(oauthConfig *oauth2.Config) *GauthHandler {
	return &GauthHandler{oauthConfig: oauthConfig}
}

func (h *GauthHandler) Login(c *gin.Context) {
	// Lax since we are using a cookie to store the verifier
	// and the callback will be called by Google (not Strict).
	c.SetSameSite(http.SameSiteLaxMode)

	verifier, err := authutil.GenerateToken()
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	callbackURL, err := url.Parse(h.oauthConfig.RedirectURL)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.SetCookie(
		/* name */ verifierCookieName,
		/* value */ verifier,
		/* maxAge */ 5*60, // 5 min
		/* path */ callbackURL.Path,
		/* domain */ "",
		/* secure */ true,
		/* httpOnly */ true,
	)

	redirectURL := h.oauthConfig.AuthCodeURL(
		"",
		oauth2.AccessTypeOnline,
		oauth2.S256ChallengeOption(verifier),
	)

	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func (h *GauthHandler) Callback(c *gin.Context) {
	verifier, err := c.Cookie(verifierCookieName)
	if err != nil {
		_ = c.AbortWithError(http.StatusUnauthorized, err)
		return
	}

	token, err := h.oauthConfig.Exchange(c.Request.Context(), c.Query("code"), oauth2.VerifierOption(verifier))
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	client, err := googleoauth2.NewService(
		c.Request.Context(),
		option.WithTokenSource(h.oauthConfig.TokenSource(c.Request.Context(), token)),
	)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	user, err := client.Userinfo.Get().Do()
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// FIXME: WIP
	c.JSON(http.StatusOK, user)
}
