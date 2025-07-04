package auth

// import (
// 	"context"
// 	"net/http"
// 	"strings"
// )

// // Middleware decodes the share session cookie and packs the session into context
// func Middleware(storage Storage) func(http.Handler) http.Handler {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			// Get Bearer authHeaderContent from header.
// 			authHeaderContent := r.Header.Get("Authorization")
// 			if authHeaderContent == "" {
// 				http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 				return
// 			}

// 			token, ok := strings.CutPrefix(authHeaderContent, "Bearer ")
// 			if !ok {
// 				http.Error(w, "Bad token. Please refresh and try again.", http.StatusUnauthorized)
// 				return
// 			}

// 			// Check if the token is valid
// 			tokenInfo, err := storage.Get(r.Context(), token)
// 			if err != nil {
// 				http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 				return
// 			}

// 			// put it in context
// 			ctx := context.WithValue(r.Context(), userCtxKey, token) /* fixme */

// 			// and call the next with our new context
// 			r = r.WithContext(ctx)
// 			next.ServeHTTP(w, r)
// 		})
// 	}
// }
