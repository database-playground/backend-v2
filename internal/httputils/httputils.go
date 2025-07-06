// Package httputils provides utilities for HTTP requests.
package httputils

import (
	"context"

	"github.com/gin-gonic/gin"
)

// httputilsContextKey is the key for the User-Agent header in the context.
type httputilsContextKey string

const (
	// contextKeyMachine is the key for the machine name in the context.
	contextKeyMachine httputilsContextKey = "httputils:machine"
)

// MachineMiddleware puts the User-Agent header into the context.
func MachineMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		newCtx := context.WithValue(c.Request.Context(), contextKeyMachine, c.GetHeader("User-Agent"))
		c.Request = c.Request.WithContext(newCtx)
		c.Next()
	}
}

// GetMachineName returns the machine name from the context.
func GetMachineName(ctx context.Context) string {
	if machine, ok := ctx.Value(contextKeyMachine).(string); ok {
		return machine
	}

	return "!not-standard-path!"
}
