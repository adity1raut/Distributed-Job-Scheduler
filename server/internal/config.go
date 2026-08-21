package config

import (
	"os"
	"strconv"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	DatabaseURL       string
	RedisAddr         string
	JWTSecret         string
	APIPort           string
	WorkerPollMS      int
	WorkerConcurrency int
	HeartbeatSec      int
	StaleJobSec       int
}

func Load() *Config {
	return &Config{
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/jobscheduler?sslmode=disable"),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		JWTSecret:         getEnv("JWT_SECRET", ""),
		APIPort:           getEnv("API_PORT", "8080"),
		WorkerPollMS:      getEnvInt("WORKER_POLL_MS", 500),
		WorkerConcurrency: getEnvInt("WORKER_CONCURRENCY", 10),
		HeartbeatSec:      getEnvInt("HEARTBEAT_SEC", 10),
		StaleJobSec:       getEnvInt("STALE_JOB_SEC", 60),
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
