package authservice

import (
	"net/http"

	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/ent/user"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/gin-gonic/gin"
)

func (s *AuthService) AuthAsAdmin(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)

	// Find the first admin user
	user, err := s.ent.User.Query().Where(user.HasGroupWith(group.Name("admin"))).First(c.Request.Context())
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	token, err := s.storage.Create(c.Request.Context(), auth.TokenInfo{
		Machine:   c.Request.UserAgent(),
		UserID:    user.ID,
		UserEmail: user.Email,
		Scopes:    []string{"*"},
	})
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
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
