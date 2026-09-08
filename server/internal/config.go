package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	DatabaseURL        string
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	JWTSecret          string
	JWTExpiryHours     int
	APIPort            string
	WorkerOrgID        string
	WorkerPollMS       int
	WorkerConcurrency  int
	HeartbeatSec       int
	StaleJobSec        int
	SchedulerTickSec   int
	RateLimitPerMin    int
	CORSAllowedOrigins []string
	TrustedProxies     []string
	ShutdownTimeoutSec int
	AppEnv             string
}

func Load() *Config {
	return &Config{
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/jobscheduler?sslmode=disable"),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            getEnvInt("REDIS_DB", 0),
		JWTSecret:          getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTExpiryHours:     getEnvInt("JWT_EXPIRY_HOURS", 24),
		APIPort:            getEnv("API_PORT", "8080"),
		WorkerOrgID:        getEnv("WORKER_ORG_ID", ""),
		WorkerPollMS:       getEnvInt("WORKER_POLL_MS", 500),
		WorkerConcurrency:  getEnvInt("WORKER_CONCURRENCY", 10),
		HeartbeatSec:       getEnvInt("HEARTBEAT_SEC", 10),
		StaleJobSec:        getEnvInt("STALE_JOB_SEC", 60),
		SchedulerTickSec:   getEnvInt("SCHEDULER_TICK_SEC", 5),
		RateLimitPerMin:    getEnvInt("RATE_LIMIT_PER_MIN", 120),
		CORSAllowedOrigins: strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173"), ","),
		// Empty by default: honouring X-Forwarded-For from an untrusted peer
		// lets any client forge its own address and escape per-IP rate
		// limiting. Set this only to the proxies actually in front of the API.
		TrustedProxies:     strings.Split(getEnv("TRUSTED_PROXIES", ""), ","),
		ShutdownTimeoutSec: getEnvInt("SHUTDOWN_TIMEOUT_SEC", 15),
		AppEnv:             getEnv("APP_ENV", "development"),
	}
}

// RedisOptions builds the client options both binaries use, so an
// authenticated or non-default-database Redis is configured in exactly one
// place rather than drifting between cmd/api and any future consumer.
func (c *Config) RedisOptions() *redis.Options {
	return &redis.Options{
		Addr:     c.RedisAddr,
		Password: c.RedisPassword,
		DB:       c.RedisDB,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
