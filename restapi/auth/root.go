// authservice provides authentication and authorization services.
package authservice

import (
	"fmt"

	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/restapi"
	"github.com/gin-gonic/gin"
)

type AuthService struct {
	storage auth.Storage
	config  config.Config
}

func NewAuthService(storage auth.Storage, config config.Config) *AuthService {
	return &AuthService{storage: storage, config: config}
}

func (s *AuthService) Register(router gin.IRouter) {
	auth := router.Group("/auth")
	auth.POST("/logout", s.Logout)

	// FIXME: remove this group in production
	{
		debug := auth.Group("/debug")
		debug.POST("/auth-as-admin", s.AuthAsAdmin)
	}

	{
		gauth := auth.Group("/google")

		oauthConfig := BuildOAuthConfig(s.config.GAuth)
		oauthConfig.RedirectURL = fmt.Sprintf("%s%s/google/callback", s.config.ServerURI, auth.BasePath())
		gauthHandler := NewGauthHandler(oauthConfig)

		gauth.GET("/login", gauthHandler.Login)
		gauth.GET("/callback", gauthHandler.Callback)
	}
}

var _ restapi.Service = (*AuthService)(nil)
