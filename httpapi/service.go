// Package httpapi provides the HTTP API controllers that does not covered by GraphQL.
//
// For the handlers of GraphQL API, see `graph/*.resolvers.go`.
package httpapi

import (
	"github.com/gin-gonic/gin"
)

// Service is the interface that should be registered to the router.
type Service interface {
	// Register registers the service with the given router.
	Register(app gin.IRouter)
}

// Register registers the services with the given router.
func Register(app gin.IRouter, services ...Service) {
	for _, service := range services {
		service.Register(app)
	}
}
