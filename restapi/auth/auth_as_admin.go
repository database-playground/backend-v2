package authservice

import (
	"net/http"

	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/gin-gonic/gin"
)

func (s *AuthService) AuthAsAdmin(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)

	token, err := s.storage.Create(c.Request.Context(), auth.TokenInfo{
		Machine: c.Request.UserAgent(),
		User:    "1",
		Scopes:  []string{"*"},
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	c.SetCookie(
		/* name */ auth.CookieAuthToken,
		/* value */ token,
		/* maxAge */ auth.DefaultTokenExpire,
		/* path */ "/",
		/* domain */ "",
		/* secure */ true,
		/* httpOnly */ true,
	)

	c.Status(http.StatusOK)
}
