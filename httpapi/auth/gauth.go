package authservice

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/authutil"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/internal/httputils"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleoauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

const (
	verifierCookieName = "Gauth-Verifier"
	redirectCookieName = "Gauth-Redirect"
)

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
	oauthConfig  *oauth2.Config
	useraccount  *useraccount.Context
	redirectURIs []string
}

func NewGauthHandler(oauthConfig *oauth2.Config, useraccount *useraccount.Context, redirectURIs []string) *GauthHandler {
	return &GauthHandler{oauthConfig: oauthConfig, useraccount: useraccount, redirectURIs: redirectURIs}
}

func (h *GauthHandler) Login(c *gin.Context) {
	// Lax since we are using a cookie to store the verifier
	// and the callback will be called by Google (not Strict).
	c.SetSameSite(http.SameSiteLaxMode)

	verifier, err := authutil.GenerateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "failed to generate verifier",
			"detail": err.Error(),
		})
		return
	}

	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		redirectURI = h.oauthConfig.RedirectURL
	}

	callbackURL, err := url.Parse(h.oauthConfig.RedirectURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "failed to parse redirect URL",
			"detail": err.Error(),
		})
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

	c.SetCookie(
		/* name */ redirectCookieName,
		/* value */ redirectURI,
		/* maxAge */ 5*60, // 5 min
		/* path */ "/",
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
	c.SetSameSite(http.SameSiteStrictMode)

	verifier, err := c.Cookie(verifierCookieName)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":  "failed to get verifier",
			"detail": err.Error(),
		})
		return
	}

	oauthToken, err := h.oauthConfig.Exchange(c.Request.Context(), c.Query("code"), oauth2.VerifierOption(verifier))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "failed to exchange code",
			"detail": err.Error(),
		})
		return
	}

	client, err := googleoauth2.NewService(
		c.Request.Context(),
		option.WithTokenSource(h.oauthConfig.TokenSource(c.Request.Context(), oauthToken)),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "failed to create google client",
			"detail": err.Error(),
		})
		return
	}

	user, err := client.Userinfo.Get().Do()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "failed to get user info",
			"detail": err.Error(),
		})
		return
	}

	entUser, err := h.useraccount.GetOrRegister(c.Request.Context(), useraccount.UserRegisterRequest{
		Email:  user.Email,
		Name:   user.Name,
		Avatar: user.Picture,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "failed to get or register user",
			"detail": err.Error(),
		})
		return
	}

	// grant verification scope to the user
	machineName := httputils.GetMachineName(c.Request.Context())
	token, err := h.useraccount.GrantToken(c.Request.Context(), entUser, machineName, useraccount.WithFlow("gauth"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "failed to grant token",
			"detail": err.Error(),
		})
		return
	}

	// write to cookie
	c.SetCookie(
		/* name */ auth.CookieAuthToken,
		/* value */ token,
		/* maxAge */ auth.DefaultTokenExpire,
		/* path */ "/",
		/* domain */ "",
		/* secure */ true,
		/* httpOnly */ true,
	)

	// redirect to the original redirect URL
	redirectURL, err := c.Cookie(redirectCookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "failed to get redirect URL",
			"detail": err.Error(),
		})
		return
	}

	// check if the redirect URL is in the allowed redirect URIs
	userRedirectURL, err := url.Parse(redirectURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "failed to parse redirect URL",
			"detail": err.Error(),
		})
		return
	}

	for _, allowedRedirectURI := range h.redirectURIs {
		parsedAllowedRedirectURI, err := url.Parse(allowedRedirectURI)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":  "failed to parse allowed redirect URI",
				"detail": err.Error(),
			})
			return
		}

		matched := userRedirectURL.Scheme == parsedAllowedRedirectURI.Scheme &&
			userRedirectURL.Host == parsedAllowedRedirectURI.Host &&
			userRedirectURL.Path == parsedAllowedRedirectURI.Path

		if matched {
			c.Redirect(http.StatusTemporaryRedirect, parsedAllowedRedirectURI.String())
			return
		}
	}

	c.JSON(http.StatusBadRequest, gin.H{
		"error":  "redirect URL is not allowed",
		"detail": fmt.Sprintf("redirect URL is not allowed: %s", redirectURL),
	})
}
