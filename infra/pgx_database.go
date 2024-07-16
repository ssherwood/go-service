package infra

import (
	"context"
	"fmt"
	"github.com/yugabyte/pgx/v5"
	"github.com/yugabyte/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"log"
	"strings"
	"time"
)

func convertMapToString(params map[string]string) string {
	var pairs []string
	for key, value := range params {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(pairs, "&")
}

func pgxConfig() *pgxpool.Config {

	configOptions := map[string]string{
		"load_balance":  "false",
		"topology_keys": "gcp.us-east1.*:1,gcp.us-central1.*:2,gcp.us-west1.*:3",
	}
	//fmt.Println(convertMapToString(configOptions))

	url := fmt.Sprintf("postgres://%s:%s@%s/%s?%s", "yugabyte", "", "127.0.0.1:5433,127.0.0.2:5433,127.0.0.3:5433", "yugabyte", convertMapToString(configOptions))
	dbConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Fatal("Failed to create a config, error: ", err)
	}

	dbConfig.MaxConns = 10
	dbConfig.MinConns = 10
	dbConfig.MaxConnLifetime = time.Hour * 4
	dbConfig.MaxConnLifetimeJitter = time.Minute * 15
	//dbConfig.MaxConnIdleTime = defaultMaxConnIdleTime
	dbConfig.HealthCheckPeriod = time.Minute * 10
	dbConfig.ConnConfig.ConnectTimeout = time.Second * 5
	dbConfig.ConnConfig.Tracer = NewQueryTracer([]attribute.KeyValue{
		semconv.DBSystemKey.String("yugabytedb"),
		semconv.DBConnectionStringKey.String(url),
	})

	dbConfig.BeforeAcquire = func(ctx context.Context, c *pgx.Conn) bool {
		log.Println("Before acquiring a database connection from the pool")

		var value string
		_ = c.QueryRow(ctx, "select current_setting('yb_read_from_followers')").Scan(&value)
		log.Println("yb_read_from_followers is", value)

		return true
	}

	dbConfig.AfterRelease = func(c *pgx.Conn) bool {
		log.Println("After releasing database connection back to the pool")

		var value string
		_ = c.QueryRow(context.Background(), "select current_setting('yb_read_from_followers')").Scan(&value)
		log.Println("yb_read_from_followers is", value)

		return true
	}

	dbConfig.BeforeClose = func(c *pgx.Conn) {
		log.Println("Closed database connection to host", c.Config().Host)
	}

	return dbConfig
}

func InitializeDB(ctx context.Context) (*pgxpool.Pool, error) {
	if dbPool, err := pgxpool.NewWithConfig(ctx, pgxConfig()); err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
		return nil, err
	} else {
		return dbPool, nil
	}
}
