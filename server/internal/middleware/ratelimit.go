package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/adity1raut/job-scheduler/internal/apperr"
	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/redis/go-redis/v9"
)

// RateLimit enforces a fixed-window request budget per organization (or per
// IP for unauthenticated requests), shared across every API replica via
// Redis — an in-process limiter would let each replica allow the full quota
// independently.
func RateLimit(rdb *redis.Client, perMinute int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := "ratelimit:" + rateLimitSubject(r)

			ctx := r.Context()
			count, err := incrWithWindow(ctx, rdb, key, time.Minute)
			if err != nil {
				// Redis being unavailable should degrade to "allow", not take the API down.
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(perMinute))
			remaining := perMinute - int(count)
			if remaining < 0 {
				remaining = 0
			}
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if int(count) > perMinute {
				httpx.WriteError(w, RequestIDFromContext(ctx), apperr.TooManyRequests("rate limit exceeded, try again shortly"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitSubject(r *http.Request) string {
	if claims := ClaimsFromContext(r.Context()); claims != nil {
		return "org:" + claims.OrgID.String()
	}
	return "ip:" + r.RemoteAddr
}

// incrWithWindow increments key and returns the new count. The window's TTL
// is set only on the first request that creates the key, not on every call
// — otherwise continuous traffic (e.g. dashboard polling) would keep pushing
// the expiry forward forever and the "fixed window" would never actually
// reset.
func incrWithWindow(ctx context.Context, rdb *redis.Client, key string, window time.Duration) (int64, error) {
	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		if err := rdb.Expire(ctx, key, window).Err(); err != nil {
			return 0, err
		}
	}
	return count, nil
}
