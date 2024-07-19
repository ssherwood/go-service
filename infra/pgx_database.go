package infra

import (
	"context"
	"fmt"
	"github.com/yugabyte/pgx/v5"
	"github.com/yugabyte/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"locationservice/config"
	"log"
	"regexp"
	"strings"
)

func InitializeDB(ctx context.Context) (*pgxpool.Pool, error) {
	poolConfig, err := pgxPoolConfig()
	if err != nil {
		return nil, err
	}

	if dbPool, err := pgxpool.NewWithConfig(ctx, poolConfig); err != nil {
		log.Printf("Unable to create pgx connection pool: %v\n", err)
		return nil, err
	} else {
		_ = initPgxPoolMeter(dbPool)
		return dbPool, nil
	}
}

func pgxPoolConfig() (*pgxpool.Config, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s/%s?%s",
		config.DBUserName, config.DBPassword, config.DBHostname, config.DBDatabase,
		mapToOptions(
			map[string]string{
				"sslmode":       config.DBSSLMode,
				"load_balance":  config.DBYSQLLoadBalance,
				"topology_keys": config.DBYSQLTopologyKeys,
			},
		),
	)

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Printf("Failed to parse pgxpool url: %v\n", err)
		return nil, err
	}

	poolConfig.MaxConns = config.DBMaxConns
	poolConfig.MinConns = config.DBMinConns
	poolConfig.MaxConnLifetime = config.DBMaxConnLifetime
	poolConfig.MaxConnLifetimeJitter = config.DBMaxConnLifetimeJitter
	poolConfig.HealthCheckPeriod = config.DBHealthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = config.DBConnectTimeout

	poolConfig.BeforeAcquire = defaultBeforeAcquireFn()
	poolConfig.AfterRelease = defaultAfterReleaseFn()
	poolConfig.BeforeClose = defaultBeforeCloseFn()

	if config.OTELTracerEnabled {
		poolConfig.ConnConfig.Tracer = NewQueryTracer([]attribute.KeyValue{
			semconv.DBSystemKey.String("yugabytedb"),
			semconv.DBConnectionStringKey.String(maskPostgresPassword(url)),
			semconv.ServerAddress(config.Hostname),
		})
	}

	return poolConfig, nil
}

func defaultBeforeAcquireFn() func(ctx context.Context, c *pgx.Conn) bool {
	return func(ctx context.Context, c *pgx.Conn) bool {
		log.Println("Before acquiring a database connection from the pool")

		var value string
		_ = c.QueryRow(ctx, "select current_setting('yb_read_from_followers')").Scan(&value)
		log.Println("yb_read_from_followers is", value)

		return true
	}
}

func defaultAfterReleaseFn() func(c *pgx.Conn) bool {
	return func(c *pgx.Conn) bool {
		log.Println("After releasing database connection back to the pool")

		var value string
		_ = c.QueryRow(context.Background(), "select current_setting('yb_read_from_followers')").Scan(&value)
		log.Println("yb_read_from_followers is", value)

		return true
	}
}

func defaultBeforeCloseFn() func(c *pgx.Conn) {
	return func(c *pgx.Conn) {
		log.Println("Closed database connection to host", c.Config().Host)
	}
}

func mapToOptions(params map[string]string) string {
	var pairs []string
	for key, value := range params {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(pairs, "&")
}

func maskPostgresPassword(connURL string) string {
	re := regexp.MustCompile(`(postgres://[^:]+:)([^@]+)(@.+)`)
	return re.ReplaceAllString(connURL, `${1}*****${3}`)
}

// initPgxPoolMeter
// given a pgxpool.Pool, build a databaseMeter callback for the available statistics available.
func initPgxPoolMeter(dbPool *pgxpool.Pool) error {
	var databaseMeter = otel.Meter("github.com/yugabyte/pgx/v5/pgxpool",
		metric.WithInstrumentationAttributes(semconv.ServiceName(config.ServiceName)))

	idleConns, _ := databaseMeter.Int64ObservableGauge("pgxpool.idleConns")
	totalConns, _ := databaseMeter.Int64ObservableGauge("pgxpool.totalConns")
	acquireCount, _ := databaseMeter.Int64ObservableCounter("pgxpool.acquireCount")
	newConnsCount, _ := databaseMeter.Int64ObservableCounter("pgxpool.newConnsCount")
	maxLifetimeDestroyCount, _ := databaseMeter.Int64ObservableCounter("pgxpool.maxLifetimeDestroyCount")
	acquireDuration, _ := databaseMeter.Int64ObservableGauge("pgxpool.acquireDuration", metric.WithUnit("ms"))

	_, err := databaseMeter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			dbStats := dbPool.Stat()
			o.ObserveInt64(idleConns, int64(dbStats.IdleConns()))
			o.ObserveInt64(totalConns, int64(dbStats.TotalConns()))
			o.ObserveInt64(acquireCount, dbStats.AcquireCount())
			o.ObserveInt64(newConnsCount, dbStats.NewConnsCount())
			o.ObserveInt64(maxLifetimeDestroyCount, dbStats.MaxLifetimeDestroyCount())
			o.ObserveInt64(acquireDuration, dbStats.AcquireDuration().Milliseconds())
			return nil
		},
		idleConns, totalConns, acquireCount, newConnsCount, maxLifetimeDestroyCount, acquireDuration,
	)
	if err != nil {
		fmt.Printf("failed to register pgxpool stats: %v\n", err)
		return err
	}

	return nil
}
