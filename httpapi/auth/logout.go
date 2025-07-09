package authservice

import (
	"net/http"

	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/gin-gonic/gin"
)

func (s *AuthService) Logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)

	// check if there is a token in the cookie
	token, err := c.Cookie(auth.CookieAuthToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "You should be logged in to logout.",
		})

		return
	}

	// revoke the token
	if err := s.storage.Delete(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to revoke the token. Please try again later.",
			"detail": err.Error(),
		})

		return
	}

	// clear the cookie from session
	c.SetCookie(
		/* name */ auth.CookieAuthToken,
		/* value */ "",
		/* maxAge */ -1,
		/* path */ "/",
		/* domain */ "",
		/* secure */ true,
		/* httpOnly */ true,
	)

	c.Status(http.StatusResetContent)
}
