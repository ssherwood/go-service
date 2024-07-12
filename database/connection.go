package database

import (
	"context"
	"fmt"
	"github.com/yugabyte/pgx/v5"
	"github.com/yugabyte/pgx/v5/pgxpool"
	"log"
	"strings"
	"time"
)

type LogTracer struct {
}

func (tracer *LogTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	log.Println(">>> Executing command:", "sql:", data.SQL, "args:", data.Args)
	return ctx
}

func (tracer *LogTracer) TraceQueryEnd(_ context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	log.Println(">>> Finishing command:", "tag:", data.CommandTag)
}

func (tracer *LogTracer) TraceConnectStart(ctx context.Context, data pgx.TraceConnectStartData) context.Context {
	log.Println(">>> ConnectionStart:", "connString:", data.ConnConfig.ConnString())
	return ctx
}

func (tracer *LogTracer) TraceConnectEnd(_ context.Context, data pgx.TraceConnectEndData) {
	if data.Err != nil {
		log.Println(">>> ConnectionEnd:", "Err:", data.Err)
	}
}

func convertMapToString(params map[string]string) string {
	var pairs []string
	for key, value := range params {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(pairs, "&")
}

func Config() *pgxpool.Config {

	configOptions := map[string]string{
		"load_balance":  "true",
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

	tracer := &LogTracer{}
	dbConfig.ConnConfig.Tracer = tracer

	dbConfig.BeforeAcquire = func(ctx context.Context, c *pgx.Conn) bool {
		log.Println("Before acquiring the connection pool to the database!!")

		var value string
		_ = c.QueryRow(context.Background(), "select current_setting('yb_read_from_followers')").Scan(&value)
		log.Println("yb_read_from_followers is", value)

		return true
	}

	dbConfig.AfterRelease = func(c *pgx.Conn) bool {
		log.Println("After releasing the connection pool to the database!!")

		var value string
		_ = c.QueryRow(context.Background(), "select current_setting('yb_read_from_followers')").Scan(&value)
		log.Println("yb_read_from_followers is", value)

		return true
	}

	dbConfig.BeforeClose = func(c *pgx.Conn) {
		log.Println("Closed the connection pool to the database!!")
	}

	return dbConfig
}

func ConnectDB() (*pgxpool.Pool, error) {
	dbPool, err := pgxpool.NewWithConfig(context.Background(), Config())
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
		return nil, err
	}

	return dbPool, nil
}
