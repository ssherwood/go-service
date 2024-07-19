package config

import (
	"os"
	"strconv"
	"time"
)

var (
	Hostname, _    = os.Hostname()
	ServiceName    = "LocationService"
	ServiceVersion = "1.0"
	ServerAddress  = GetEnv("SERVER_ADDRESS", ":8080")
	WriteTimeout   = GetEnvAsDuration("WRITE_TIMEOUT", 15*time.Second)
	ReadTimeout    = GetEnvAsDuration("READ_TIMEOUT", 10*time.Second)
	CollectorURL   = GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	Insecure       = GetEnv("INSECURE_MODE", "true")
)

func GetEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func GetEnvAsInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		} else {
			return fallback
		}
	}
	return fallback
}

func GetEnvAsDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if intValue, err := strconv.Atoi(value); err == nil {
			return time.Duration(intValue)
		} else {
			return fallback
		}
	}
	return fallback
}
