// authservice provides authentication and authorization services.
package authservice

import (
	"fmt"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/httpapi"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/gin-gonic/gin"
)

type AuthService struct {
	entClient *ent.Client
	storage   auth.Storage
	config    config.Config
}

func NewAuthService(entClient *ent.Client, storage auth.Storage, config config.Config) *AuthService {
	return &AuthService{entClient: entClient, storage: storage, config: config}
}

func (s *AuthService) Register(router gin.IRouter) {
	auth := router.Group("/auth")
	auth.POST("/logout", s.Logout)

	{
		gauth := auth.Group("/google")

		oauthConfig := BuildOAuthConfig(s.config.GAuth)
		oauthConfig.RedirectURL = fmt.Sprintf("%s%s/google/callback", s.config.Server.URI, auth.BasePath())

		useraccount := useraccount.NewContext(s.entClient, s.storage)

		gauthHandler := NewGauthHandler(oauthConfig, useraccount, s.config.GAuth.RedirectURL)

		gauth.GET("/login", gauthHandler.Login)
		gauth.GET("/callback", gauthHandler.Callback)
	}
}

var _ httpapi.Service = (*AuthService)(nil)
