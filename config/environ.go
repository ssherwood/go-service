package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

var (
	Hostname, _               = os.Hostname()
	ServiceName               = "LocationService"
	ServiceVersion            = "1.0"
	ServerAddress             = GetEnv("SERVER_ADDRESS", ":8080")
	ServerWriteTimeout        = GetEnv("SERVER_WRITE_TIMEOUT", 15*time.Second)
	ServerReadTimeout         = GetEnv("SERVER_READ_TIMEOUT", 10*time.Second)
	DBUserName                = GetEnv("DB_USERNAME", "yugabyte")
	DBPassword                = GetEnv("DB_PASSWORD", "")
	DBHostname                = GetEnv("DB_HOSTNAME", "127.0.0.1:5433,127.0.0.2:5433,127.0.0.3:5433")
	DBDatabase                = GetEnv("DB_DATABASE", "yugabyte")
	DBSSLMode                 = GetEnv("DB_SSL_MODE", "disable")
	DBYSQLLoadBalance         = GetEnv("DB_YSQL_LOAD_BALANCE", "true")
	DBYSQLTopologyKeys        = GetEnv("DB_YSQL_TOPOLOGY_KEYS", "gcp.us-east1.*:1,gcp.us-central1.*:2,gcp.us-west1.*:3")
	DBMaxConns                = GetEnv("DB_MAX_CONNS", int32(10))
	DBMinConns                = GetEnv("DB_MIN_CONNS", int32(10))
	DBMaxConnLifetime         = GetEnv("DB_MAX_CONN_LIFETIME", 4*time.Hour)
	DBMaxConnLifetimeJitter   = GetEnv("DB_MAX_CONN_LIFETIME_JITTER", 15*time.Minute)
	DBHealthCheckPeriod       = GetEnv("DB_HEALTH_CHECK_PERIOD", 10*time.Minute)
	DBConnectTimeout          = GetEnv("DB_CONNECT_TIMEOUT", 5*time.Second)
	OTELCollectorURL          = GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	OTELExporterInsecure      = GetEnv("OTEL_EXPORTER_INSECURE_MODE", "true")
	OTELCompressor            = GetEnv("OTEL_GRPC_COMPRESSOR", "gzip")
	OTELMeterInterval         = GetEnv("OTEL_METRIC_POLL_INTERVAL", 15*time.Second)
	OTELTracerEnabled         = GetEnv("OTEL_TRACER_ENABLE", true)
	OTELTracerLogSQLStatement = GetEnv("OTEL_TRACER_LOG_SQL_STMT", true)
	OTELTracerIncludeParams   = GetEnv("OTEL_TRACER_INCLUDE_PARAMS", true)
	OTELPrefixQuerySpanName   = GetEnv("OTEL_PREFIX_QUERY_SPAN_NAME", true)
)

func GetEnv[T any](key string, defaultValue T) T {
	if value, exists := os.LookupEnv(key); !exists {
		return defaultValue
	} else {
		switch any(defaultValue).(type) {
		case string:
			return any(value).(T)
		case int:
			if intValue, err := strconv.Atoi(value); err == nil {
				return any(intValue).(T)
			}
		case int64:
			if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
				return any(intValue).(T)
			}
		case uint:
			if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
				return any(uintValue).(T)
			}
		case float32:
			if floatValue, err := strconv.ParseFloat(value, 32); err == nil {
				return any(float32(floatValue)).(T)
			}
		case float64:
			if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
				return any(floatValue).(T)
			}
		case bool:
			if boolValue, err := strconv.ParseBool(value); err == nil {
				return any(boolValue).(T)
			}
		case time.Duration:
			if durationValue, err := time.ParseDuration(value); err == nil {
				return any(durationValue).(T)
			}
		}

		log.Printf("GetEnv unable process type %T from '%s=%s', reverting to defaultValue",
			defaultValue, key, value)
	}

	return defaultValue
}
