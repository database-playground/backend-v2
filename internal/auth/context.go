package auth

import "context"

type authContextKey string

const (
	userContextKey authContextKey = "user"
)

func WithUser(ctx context.Context, info TokenInfo) context.Context {
	return context.WithValue(ctx, userContextKey, info)
}

func GetUser(ctx context.Context) (TokenInfo, bool) {
	info, ok := ctx.Value(userContextKey).(TokenInfo)
	return info, ok
}
