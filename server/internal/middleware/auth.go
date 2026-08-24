package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/adity1raut/job-scheduler/internal/apperr"
	"github.com/adity1raut/job-scheduler/internal/authtoken"
	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
)

type authCtxKey int

const claimsKey authCtxKey = iota

// Auth verifies the bearer JWT on the request and attaches its claims to
// the request context for downstream handlers to read.
func Auth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				httpx.WriteError(w, RequestIDFromContext(r.Context()), apperr.Unauthorized("missing bearer token"))
				return
			}
			tokenString := strings.TrimPrefix(header, "Bearer ")

			claims, err := authtoken.Parse(secret, tokenString)
			if err != nil {
				httpx.WriteError(w, RequestIDFromContext(r.Context()), apperr.Unauthorized("invalid or expired token"))
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClaimsFromContext(ctx context.Context) *authtoken.Claims {
	claims, _ := ctx.Value(claimsKey).(*authtoken.Claims)
	return claims
}

func UserIDFromContext(ctx context.Context) uuid.UUID {
	if claims := ClaimsFromContext(ctx); claims != nil {
		return claims.UserID
	}
	return uuid.Nil
}

func OrgIDFromContext(ctx context.Context) uuid.UUID {
	if claims := ClaimsFromContext(ctx); claims != nil {
		return claims.OrgID
	}
	return uuid.Nil
}

func RoleFromContext(ctx context.Context) models.Role {
	if claims := ClaimsFromContext(ctx); claims != nil {
		return models.Role(claims.Role)
	}
	return ""
}

// RequireRole gates a route to a fixed set of roles — it must sit behind
// Auth in the middleware chain, since it reads the role Auth already
// attached to the request context.
func RequireRole(allowed ...models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := RoleFromContext(r.Context())
			for _, a := range allowed {
				if role == a {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpx.WriteError(w, RequestIDFromContext(r.Context()), apperr.Forbidden("your role does not permit this action"))
		})
	}
}
