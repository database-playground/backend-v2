package auth

import "context"

type authContextKey string

const (
	userContextKey authContextKey = "user"
)

// WithUser adds the user information to the context.
//
// For any request with this context, you can use `GetUser`
// to get the user information.
func WithUser(ctx context.Context, info TokenInfo) context.Context {
	return context.WithValue(ctx, userContextKey, info)
}

// GetUser returns the user information from the context.
//
// It returns the user information and a boolean indicating
// whether the user information is present.
func GetUser(ctx context.Context) (TokenInfo, bool) {
	info, ok := ctx.Value(userContextKey).(TokenInfo)
	return info, ok
}
