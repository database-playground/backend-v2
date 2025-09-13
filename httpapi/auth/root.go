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
	useraccount := useraccount.NewContext(s.entClient, s.storage)

	auth := router.Group("/auth/v2")
	oauthConfig := BuildOAuthConfig(s.config.GAuth)
	oauthConfig.RedirectURL = fmt.Sprintf("%s%s/callback/google", s.config.Server.URI, auth.BasePath())
	gauthHandler := NewGauthHandler(oauthConfig, useraccount, s.config.GAuth.RedirectURIs, s.config.GAuth.Secret)

	auth.GET("/authorize/google", gauthHandler.Authorize)
	auth.GET("/callback/google", gauthHandler.Callback)
	auth.POST("/token", gauthHandler.Token)
	auth.POST("/revoke", s.RevokeToken)
}

var _ httpapi.Service = (*AuthService)(nil)
