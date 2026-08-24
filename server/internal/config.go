package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	DatabaseURL        string
	RedisAddr          string
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
}

func Load() *Config {
	return &Config{
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/jobscheduler?sslmode=disable"),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
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
