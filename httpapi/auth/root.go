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
	entClient   *ent.Client
	storage     auth.Storage
	config      config.BackendConfig
	useraccount *useraccount.Context
}

func NewAuthService(entClient *ent.Client, storage auth.Storage, config config.BackendConfig, useraccount *useraccount.Context) *AuthService {
	return &AuthService{
		entClient:   entClient,
		storage:     storage,
		config:      config,
		useraccount: useraccount,
	}
}

func (s *AuthService) Register(router gin.IRouter) {
	auth := router.Group("/auth/v2")
	oauthConfig := BuildOAuthConfig(s.config.GAuth)
	oauthConfig.RedirectURL = fmt.Sprintf("%s%s/callback/google", s.config.Server.URI, auth.BasePath())
	gauthHandler := NewGauthHandler(oauthConfig, s.useraccount, s.config.GAuth.RedirectURIs, s.config.GAuth.Secret)

	auth.GET("/authorize/google", gauthHandler.Authorize)
	auth.GET("/callback/google", gauthHandler.Callback)
	auth.POST("/token", gauthHandler.Token)
	auth.POST("/revoke", s.RevokeToken)
	auth.POST("/introspect", s.IntrospectToken)
}

var _ httpapi.Service = (*AuthService)(nil)
